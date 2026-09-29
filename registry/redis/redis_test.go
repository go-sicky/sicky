package redis

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/go-sicky/sicky/registry"
)

// startFakeRedis starts a minimal RESP server that answers everything with
// +OK (and SUBSCRIBE with a confirmation frame) and reports how many TCP
// connections it accepted. Each redis.Client.Subscribe takes one dedicated
// connection, so the accepted count is exactly the number of PubSub
// subscriptions that were opened.
func startFakeRedis(t *testing.T) (addr string, accepted *atomic.Int64) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	// Allocate before the accept loop starts: a named result is only assigned
	// after the function returns, so capturing it in the goroutine captures nil.
	counted := &atomic.Int64{}

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}

			counted.Add(1)

			go serveFakeRedisConn(c)
		}
	}()

	return ln.Addr().String(), counted
}

// helloUnsupportedReply makes go-redis conclude that the server predates
// HELLO. initConn treats a redis error reply as "this server does not support
// HELLO" and falls back to RESP2, which is the path this fake wants. A
// successful-looking but wrong-shaped reply is not a redis error, so the
// client closes the connection and reconnects in a tight loop, which would
// swamp the connection count this test measures.
const helloUnsupportedReply = "-ERR unknown command 'HELLO'\r\n"

func serveFakeRedisConn(c net.Conn) {
	defer func() { _ = c.Close() }()

	r := bufio.NewReader(c)
	w := bufio.NewWriter(c)

	for {
		args, err := readRESPCommand(r)
		if err != nil {
			return
		}

		if len(args) == 0 {
			continue
		}

		switch strings.ToUpper(args[0]) {
		case "HELLO":
			_, _ = w.WriteString(helloUnsupportedReply)
		case "SUBSCRIBE":
			for i, ch := range args[1:] {
				_, _ = w.WriteString("*3\r\n$9\r\nsubscribe\r\n$" + strconv.Itoa(len(ch)) + "\r\n" + ch + "\r\n:" + strconv.Itoa(i+1) + "\r\n")
			}
		case "PING":
			_, _ = w.WriteString("+PONG\r\n")
		default:
			_, _ = w.WriteString("+OK\r\n")
		}

		if err := w.Flush(); err != nil {
			return
		}
	}
}

// respArrayLen parses the element count out of a RESP array header line.
func respArrayLen(line string) (int, bool) {
	if line == "" || line[0] != '*' {
		return 0, false
	}

	n, err := strconv.Atoi(line[1:])
	if err != nil {
		return 0, false
	}

	return n, true
}

// readRESPCommand reads one RESP array of bulk strings. Anything else is
// reported as an empty command, which the caller skips.
func readRESPCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}

	line = strings.TrimRight(line, "\r\n")

	n, ok := respArrayLen(line)
	if !ok {
		// Not a RESP array (or not one this fake models): skip it rather
		// than failing the connection.
		return nil, nil
	}

	if n <= 0 {
		return nil, nil
	}

	args := make([]string, 0, n)

	for range n {
		head, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}

		size, err := strconv.Atoi(strings.TrimRight(head, "\r\n")[1:])
		if err != nil {
			return nil, err
		}

		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}

		args = append(args, string(buf[:size]))
	}

	return args, nil
}

// TestWatchIsIdempotent guards the fix for a non-idempotent Watch: the
// orchestrator calls it on the concrete type, registry.Watch() calls it again
// through the default registry, and every service calls it on each attached
// registry. Without the sync.Once guard each call opened its own PubSub
// connection and goroutine, so the process paid for a permanent duplicate
// HGETALL + PurgePool on every discovery change.
func TestWatchIsIdempotent(t *testing.T) {
	addr, accepted := startFakeRedis(t)

	rg := &Redis{
		config:  (&Config{Addr: addr}).Ensure(),
		ctx:     t.Context(),
		options: (&registry.Options{}).Ensure(),
		client: redis.NewClient(&redis.Options{
			Addr:            addr,
			Protocol:        2,
			DisableIdentity: true,
		}),
	}
	t.Cleanup(func() { _ = rg.client.Close() })

	for i := range 3 {
		if err := rg.Watch(); err != nil {
			t.Fatalf("Watch call %d: %v", i+1, err)
		}
	}

	// Wait until the accepted count stops moving so a late connection cannot
	// hide behind the assertion, then require exactly one subscription.
	deadline := time.Now().Add(2 * time.Second)
	last := int64(-1)
	stableFor := 0

	for time.Now().Before(deadline) {
		cur := accepted.Load()
		if cur == last && cur > 0 {
			stableFor++

			if stableFor >= 3 {
				break
			}
		} else {
			stableFor = 0
		}

		last = cur

		time.Sleep(50 * time.Millisecond)
	}

	if got := accepted.Load(); got != 1 {
		t.Fatalf("Watch() opened %d PubSub connections, want exactly 1: "+
			"a repeated Watch must be a no-op, not a second subscriber", got)
	}
}

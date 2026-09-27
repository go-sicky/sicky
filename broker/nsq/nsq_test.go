package nsq

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	nsqio "github.com/nsqio/go-nsq"

	"github.com/go-sicky/sicky/broker"
)

// startFakeNSQD speaks just enough of the nsqd TCP protocol for
// ConnectToNSQD to succeed: magic, IDENTIFY (framed) and SUB. The real
// broker is not available in unit tests, and a failing dial would make
// every Subscribe insert nothing - hiding the very race this guards.
func startFakeNSQD(t *testing.T) (addr string, stop func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	var wg sync.WaitGroup
	done := make(chan struct{})

	wg.Go(func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}

			go serveFakeNSQD(conn, done)
		}
	})

	return ln.Addr().String(), func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
	}
}

func serveFakeNSQD(conn net.Conn, done <-chan struct{}) {
	defer func() {
		_ = conn.Close()
	}()

	reader := bufio.NewReader(conn)

	// Client magic ("  V2").
	if _, err := reader.Peek(4); err != nil {
		return
	}

	if _, err := reader.Discard(4); err != nil {
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		cmd := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(cmd, "IDENTIFY"):
			// IDENTIFY carries a length-prefixed JSON body.
			var size uint32
			if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
				return
			}

			body := make([]byte, size)
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}

			respond(conn, `{"max_rdy_count":2500,"tls_v1":false,"deflate":false,"snappy":false,"auth_required":false}`)
		case strings.HasPrefix(cmd, "SUB"):
			respond(conn, "_OK")
		}
	}
}

// respond writes one nsqd response frame: [size][frameType][payload].
func respond(conn net.Conn, payload string) {
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[0:4], 0) // FrameResponse
	copy(frame[4:], payload)

	out := make([]byte, 4+len(frame))
	binary.BigEndian.PutUint32(out[0:4], uint32(len(frame)))
	copy(out[4:], frame)

	_, _ = conn.Write(out)
}

func newTestBroker(t *testing.T, addr string) *NSQ {
	t.Helper()

	brk := New(&broker.Options{Name: "nsq-test"}, &Config{Endpoint: addr, Channel: "test-channel"})
	if brk == nil {
		t.Fatal("New returned nil (config rejected)")
	}

	return brk
}

// TestSubscribeConcurrentCreatesOneConsumer: the dup check and the insert
// used to sit in different critical sections, so concurrent Subscribe
// calls for one topic each dialed nsqd and overwrote each other in the
// map - leaking a consumer that neither Unsubscribe nor Disconnect could
// reach.
func TestSubscribeConcurrentCreatesOneConsumer(t *testing.T) {
	addr, stop := startFakeNSQD(t)
	defer stop()

	brk := newTestBroker(t, addr)
	defer func() {
		if err := brk.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	}()

	var (
		mu      sync.Mutex
		created int
	)

	brk.newConsumer = func(topic, channel string, cfg *nsqio.Config) (*nsqio.Consumer, error) {
		mu.Lock()
		created++
		mu.Unlock()

		return nsqio.NewConsumer(topic, channel, cfg)
	}

	const writers = 8

	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			_ = brk.Subscribe("topic-shared", nil)
		})
	}

	wg.Wait()

	mu.Lock()
	got := created
	mu.Unlock()

	if got != 1 {
		t.Fatalf("consumers created = %d, want 1 (the others must see the existing subscription)", got)
	}

	brk.mu.RLock()
	subs := len(brk.subscriptions)
	brk.mu.RUnlock()

	if subs != 1 {
		t.Fatalf("subscriptions = %d, want 1", subs)
	}
}

// TestSubscribeDeduplicatesSequentially: after a successful subscribe the
// next call must be a no-op, not a second dial.
func TestSubscribeDeduplicatesSequentially(t *testing.T) {
	addr, stop := startFakeNSQD(t)
	defer stop()

	brk := newTestBroker(t, addr)
	defer func() {
		if err := brk.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	}()

	var created int
	brk.newConsumer = func(topic, channel string, cfg *nsqio.Config) (*nsqio.Consumer, error) {
		created++

		return nsqio.NewConsumer(topic, channel, cfg)
	}

	if err := brk.Subscribe("topic-once", nil); err != nil {
		t.Fatalf("first subscribe: %v", err)
	}

	if err := brk.Subscribe("topic-once", nil); err != nil {
		t.Fatalf("second subscribe: %v", err)
	}

	if created != 1 {
		t.Fatalf("consumers created = %d, want 1", created)
	}
}

// TestPublishRacesWithDisconnect guards the handle snapshot: Publish read
// brk.producer without the lock while Disconnect cleared it under the
// lock. Run with -race to make the regression loud.
func TestPublishRacesWithDisconnect(t *testing.T) {
	brk := newTestBroker(t, "127.0.0.1:1")

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = brk.Publish("topic", &broker.Message{Body: []byte("x")})
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				// Disconnect nils brk.producer under the lock.
				_ = brk.Disconnect()
			}
		}
	})

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

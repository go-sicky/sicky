package jetstream

import (
	"bufio"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/go-sicky/sicky/broker"
)

// TestPublishRacesWithConnectionWrite guards the handle snapshot: Publish
// used to read brk.conn and brk.streamer without the lock while
// Disconnect cleared both under the lock. The writer below mirrors
// exactly what Disconnect does with those fields.
func TestPublishRacesWithConnectionWrite(t *testing.T) {
	brk := New(&broker.Options{Name: "jetstream-test"}, &Config{})
	if brk == nil {
		t.Fatal("New returned nil (config rejected)")
	}

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
				brk.mu.Lock()
				brk.conn = nil
				brk.streamer = nil
				brk.streamInfo = nil
				brk.mu.Unlock()
			}
		}
	})

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// startFakeNATS speaks just enough of the nats wire protocol for
// nats.Connect to complete: an INFO line, then PING/PONG. The real broker
// is not available in unit tests, and a nil conn makes every Subscribe
// short-circuit before reaching the code path this test guards.
func startFakeNATS(t *testing.T) (addr string, subSeen <-chan struct{}, stop func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	// subSeen closes when the client issues a SUB, which for
	// streamer.Subscribe is the $JS.API.CONSUMER.DURABLE.CREATE request.
	// It is the test's only reliable proof that Subscribe is parked in the
	// server round-trip: brk.pending is guarded by the very lock under test,
	// so polling it would block instead of reporting.
	sub := make(chan struct{})

	var once sync.Once
	var wg sync.WaitGroup

	done := make(chan struct{})

	wg.Go(func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}

			wg.Go(func() { serveFakeNATS(conn, done, sub, &once) })
		}
	})

	return ln.Addr().String(), sub, func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
	}
}

func serveFakeNATS(conn net.Conn, done <-chan struct{}, sub chan struct{}, once *sync.Once) {
	defer func() {
		_ = conn.Close()
	}()

	const info = "INFO {\"server_id\":\"fake\",\"version\":\"2.10.0\",\"proto\":1," +
		"\"go\":\"go1.27\",\"host\":\"127.0.0.1\",\"port\":4222,\"headers\":true,\"max_payload\":1048576}\r\n"

	if _, err := io.WriteString(conn, info); err != nil {
		return
	}

	reader := bufio.NewReader(conn)
	for {
		select {
		case <-done:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "PING"):
			if _, werr := io.WriteString(conn, "PONG\r\n"); werr != nil {
				return
			}
		case strings.HasPrefix(upper, "SUB"):
			// Signal, then never answer: the request stays in flight until
			// the connection is closed, keeping Subscribe parked.
			once.Do(func() { close(sub) })
		}
	}
}

// TestSubscribeReleasesLockBeforeServerRoundTrip: Subscribe used to hold
// brk.mu (write) across streamer.Subscribe, which issues a
// $JS.API.CONSUMER.DURABLE.CREATE and blocks for the server reply. Every
// concurrent Publish, Unsubscribe, Handle and Disconnect blocked for the
// full RTT, and a handler subscribing from a publish callback deadlocked.
func TestSubscribeReleasesLockBeforeServerRoundTrip(t *testing.T) {
	addr, subSeen, stopFake := startFakeNATS(t)
	defer stopFake()

	brk := New(&broker.Options{Name: "jetstream-lock-test"}, &Config{URL: "nats://" + addr})
	if brk == nil {
		t.Fatal("New returned nil (config rejected)")
	}

	defer func() {
		if err := brk.Disconnect(); err != nil {
			t.Errorf("disconnect: %v", err)
		}
	}()

	// Drive the same assignment Connect() performs, minus AddStream: the
	// test only needs a connected conn and a live JetStream context.
	nc, err := nats.Connect("nats://"+addr, nats.Timeout(5*time.Second))
	if err != nil {
		t.Fatalf("connect to fake nats: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream context: %v", err)
	}

	brk.mu.Lock()
	brk.conn = nc
	brk.streamer = js
	brk.mu.Unlock()

	// The fake server never answers CONSUMER.DURABLE.CREATE, so Subscribe
	// parks inside streamer.Subscribe - exactly the window the fix moved out
	// of the critical section.
	subErr := make(chan error, 1)
	go func() { subErr <- brk.Subscribe("topic-slow", nil) }()

	select {
	case <-subSeen:
		// The request is in flight; Subscribe is now blocked on the reply.
	case <-time.After(5 * time.Second):
		t.Fatal("Subscribe never reached the server round-trip")
	}

	// A reader must acquire brk.mu while the round-trip is in flight. If
	// Subscribe still held the write lock this would block, proving the
	// regression.
	acquired := make(chan struct{})
	go func() {
		brk.mu.RLock()
		close(acquired)
		brk.mu.RUnlock()
	}()

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		nc.Close()
		t.Fatal("brk.mu.RLock blocked while Subscribe was in the server round-trip: Subscribe holds the write lock across streamer.Subscribe")
	}

	// The call is still parked; closing the connection makes it return.
	nc.Close()
	<-subErr

	// The failed subscribe must have released its reservation, otherwise
	// the topic is poisoned: every later Subscribe would see it as "dup".
	brk.mu.RLock()
	_, reserved := brk.pending["topic-slow"]
	brk.mu.RUnlock()

	if reserved {
		t.Fatal("topic still reserved in brk.pending after a failed subscribe")
	}
}

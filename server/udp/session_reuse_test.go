package udp

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
)

type countingHandler struct {
	connects atomic.Int64
	datas    atomic.Int64
	first    chan struct{}
}

func (h *countingHandler) Name() string { return "counting" }
func (h *countingHandler) Type() string { return "counting" }

func (h *countingHandler) OnConnect(_ *Session) error {
	if h.connects.Add(1) == 1 {
		close(h.first)
	}

	return nil
}

func (h *countingHandler) OnData(_ *Session, _ []byte) error {
	h.datas.Add(1)

	return nil
}

// The packet loop looked its session up with GetByKey, which reads p.keys —
// an index Pool.Put only populates from sess.Key, and nothing in the framework
// ever calls SetKey on a UDP session. The lookup therefore missed on every
// datagram and the loop minted a new session each time: one session and one
// OnConnect per packet, with the pool growing at the packet rate.
//
// Put does index by address (p.addrs) and GetByAddr reads that index, so the
// fix is to ask the pool the question Put actually answered.
func TestPacketLoopReusesOneSessionPerSource(t *testing.T) {
	h := &countingHandler{first: make(chan struct{})}

	srv := New(&server.Options{Name: "reuse"}, &Config{ReadTimeout: 1})
	srv.Handle(h)

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() { _ = srv.Stop() }()

	conn := dialServer(t, srv)

	const datagrams = 5

	for range datagrams {
		if _, err := conn.Write([]byte("ping")); err != nil {
			t.Fatalf("write: %v", err)
		}

		time.Sleep(20 * time.Millisecond)
	}

	waitFirst(t, h.first)

	// Let the last packet land.
	time.Sleep(200 * time.Millisecond)

	if got := h.datas.Load(); got != datagrams {
		t.Errorf("OnData = %d, want %d", got, datagrams)
	}

	if got := h.connects.Load(); got != 1 {
		t.Errorf("OnConnect = %d, want 1: every datagram minted a fresh session", got)
	}

	if got := srv.pool.Length(); got != 1 {
		t.Errorf("pool length = %d, want 1", got)
	}
}

// The user-visible consequence: with the lookup missing, one client's own
// earlier packets filled the session cap and its later packets were dropped as
// over-cap. A legitimate client silently lost data.
func TestMaxSessionsDoesNotCapARepeatingSource(t *testing.T) {
	h := &countingHandler{first: make(chan struct{})}

	srv := New(&server.Options{Name: "cap"}, &Config{ReadTimeout: 1, MaxSessions: 2})
	srv.Handle(h)

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() { _ = srv.Stop() }()

	conn := dialServer(t, srv)

	const datagrams = 3

	for range datagrams {
		if _, err := conn.Write([]byte("ping")); err != nil {
			t.Fatalf("write: %v", err)
		}

		time.Sleep(30 * time.Millisecond)
	}

	waitFirst(t, h.first)

	time.Sleep(300 * time.Millisecond)

	if got := h.datas.Load(); got != datagrams {
		t.Errorf("OnData = %d for %d datagrams from one client, want %d: the "+
			"client's own earlier packets filled the cap and its later ones were "+
			"dropped as over-cap", got, datagrams, datagrams)
	}
}

func dialServer(t *testing.T, srv *UDPServer) *net.UDPConn {
	t.Helper()

	conn, err := net.DialUDP("udp", nil, srv.Addr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func waitFirst(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never ran")
	}
}

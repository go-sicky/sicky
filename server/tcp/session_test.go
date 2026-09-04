package tcp

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
)

type noopHandler struct{}

func (noopHandler) Name() string                      { return "noop" }
func (noopHandler) Type() string                      { return "tcp" }
func (noopHandler) OnConnect(*Session) error          { return nil }
func (noopHandler) OnClose(*Session) error            { return nil }
func (noopHandler) OnError(*Session, error) error     { return nil }
func (noopHandler) OnData(s *Session, _ []byte) error { return nil }

type countingHandler struct {
	noopHandler
	total int64
	mu    sync.Mutex
}

func (h *countingHandler) OnData(_ *Session, data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.total += int64(len(data))

	return nil
}

func (h *countingHandler) sum() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.total
}

func TestSessionSetKey(t *testing.T) {
	p := NewPool(60)
	c1, c2 := net.Pipe()
	defer c2.Close()
	s := NewSession(c1)
	p.Put(s)

	s.SetKey("alpha")
	if got := p.GetByKey("alpha"); got != s {
		t.Fatal("GetByKey(alpha) missed after SetKey")
	}

	s.SetKey("beta")
	if got := p.GetByKey("alpha"); got != nil {
		t.Fatal("old key still indexed after re-key")
	}
	if got := p.GetByKey("beta"); got != s {
		t.Fatal("GetByKey(beta) missed after re-key")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := p.GetByKey("beta"); got != nil {
		t.Fatal("key still indexed after Close")
	}
	// Idempotent: second close is a no-op.
	if err := s.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestServerMaxSessions(t *testing.T) {
	srv := New(
		&server.Options{Name: "tcp-max-sessions-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0", BufferSize: 1024, MaxSessions: 1},
	)
	srv.Handle(noopHandler{})
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		if err := srv.Stop(); err != nil {
			t.Fatalf("stop: %v", err)
		}
	}()

	addr := srv.Addr().String()
	c1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer c1.Close()

	// Give the accept loop a moment to register the session.
	deadline := time.Now().Add(2 * time.Second)
	for srv.pool.Length() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.pool.Length() != 1 {
		t.Fatalf("pool length = %d, want 1", srv.pool.Length())
	}

	c2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	defer c2.Close()

	// The over-cap connection must be rejected promptly.
	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := c2.Read(buf); err == nil {
		t.Fatal("over-cap connection not rejected")
	}
	if srv.pool.Length() != 1 {
		t.Fatalf("pool length = %d, want 1 (no leak)", srv.pool.Length())
	}
}

func TestServerMaxMessageBytes(t *testing.T) {
	h := &countingHandler{}
	srv := New(
		&server.Options{Name: "tcp-max-bytes-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0", BufferSize: 1024, MaxMessageBytes: 100},
	)
	srv.Handle(h)
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		if err := srv.Stop(); err != nil {
			t.Fatalf("stop: %v", err)
		}
	}()

	c, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	// Under the cap: dispatched.
	if _, err := c.Write(make([]byte, 60)); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.sum() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.sum(); got != 60 {
		t.Fatalf("delivered = %d, want 60", got)
	}

	// Over the cap: connection closed, excess chunk dropped.
	if _, err := c.Write(make([]byte, 60)); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("over-cap connection not closed")
	}
	if got := h.sum(); got != 60 {
		t.Fatalf("delivered = %d, want 60 (excess must be dropped)", got)
	}
}

package udp

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

type blockingHandler struct {
	entered  chan struct{}
	release  chan struct{}
	firstOne sync.Once
}

func (h *blockingHandler) Name() string { return "blocking" }
func (h *blockingHandler) Type() string { return "blocking" }

func (h *blockingHandler) OnConnect(_ *Session) error { return nil }

func (h *blockingHandler) OnData(_ *Session, _ []byte) error {
	h.firstOne.Do(func() { close(h.entered) })
	<-h.release

	return nil
}

// Stop cleared `stopping` even when the drain timed out, so a Start landing
// afterwards was accepted while the old loop was still inside a handler. That
// Start wrote srv.conn and srv.addr (read unlocked by the live loop) and
// called srv.wg.Go — a WaitGroup.Add concurrent with the Wait Stop had issued,
// which the runtime may throw on.
//
// Restarting must be refused until the loop has actually drained.
func TestStopTimeoutRefusesRestartUntilDrained(t *testing.T) {
	h := &blockingHandler{entered: make(chan struct{}), release: make(chan struct{})}

	srv := New(&server.Options{Name: "drain"}, &Config{
		ReadTimeout:     1,
		ShutdownTimeout: 1,
		MaxIdleDuration: 60,
	})
	srv.Handle(h)

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	conn := dialServer(t, srv)

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-h.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler never entered")
	}

	// The handler is parked, so the drain cannot finish inside the timeout.
	err := srv.Stop()
	if !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Stop() = %v, want one wrapping utils.ErrStopTimeout. The error "+
			"was built inline with errors.New, so a caller could not match it — "+
			"server/tcp already returns the sentinel for this exact condition", err)
	}

	// Still draining: a restart must be refused rather than racing the loop.
	if err := srv.Start(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Errorf("Start() while draining = %v, want utils.ErrStopTimeout", err)
	}

	close(h.release)

	// Once the loop is really gone, a restart is allowed again.
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if err := srv.Start(); err == nil {
			_ = srv.Stop()

			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("Start never succeeded after the loop drained")
}

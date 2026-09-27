package tcp

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

// connectBlocker parks OnConnect. It runs inline in the accept goroutine
// (see tcp.go), which is exactly what Stop's phase 1 waits for.
type connectBlocker struct {
	entered  chan struct{}
	release  chan struct{}
	finished chan struct{}
}

func (h *connectBlocker) Name() string { return "blocker" }
func (h *connectBlocker) Type() string { return "tcp" }

func (h *connectBlocker) OnConnect(*Session) error {
	select {
	case h.entered <- struct{}{}:
	default:
	}

	<-h.release
	close(h.finished)

	return nil
}

func (h *connectBlocker) OnClose(*Session) error        { return nil }
func (h *connectBlocker) OnError(*Session, error) error { return nil }
func (h *connectBlocker) OnData(*Session, []byte) error { return nil }

func (h *connectBlocker) finishedOnce() bool {
	select {
	case <-h.finished:
		return true
	default:
		return false
	}
}

// TestStopTimesOutOnBlockedOnConnect: phase 1 waited on the accept
// goroutine without a bound, so one blocking OnConnect wedged the whole
// shutdown - the ShutdownTimeout only covered phase 2.
func TestStopTimesOutOnBlockedOnConnect(t *testing.T) {
	blocker := &connectBlocker{
		entered:  make(chan struct{}, 1),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}

	srv := New(
		&server.Options{Name: "tcp-stop"},
		&Config{
			Network: "tcp",
			Address: "127.0.0.1:0",
			// Seconds (int): both phases are bounded by it.
			ShutdownTimeout: 1,
		},
	)
	srv.Handle(blocker)

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() {
		_ = conn.Close()
	}()

	select {
	case <-blocker.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("OnConnect never ran")
	}

	start := time.Now()
	stopErr := srv.Stop()
	if !errors.Is(stopErr, utils.ErrStopTimeout) {
		t.Fatalf("Stop = %v, want ErrStopTimeout", stopErr)
	}

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("Stop took %s, want ~2s (two bounded phases)", elapsed)
	}

	// Let the blocked handler finish so its goroutine can exit.
	close(blocker.release)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if blocker.finishedOnce() {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("blocked OnConnect never completed after release")
}

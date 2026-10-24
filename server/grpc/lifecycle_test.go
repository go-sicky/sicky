/**
 * @file lifecycle_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package grpc

import (
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/go-sicky/sicky/server"
)

// accepts reports whether addr takes a TCP connection. The probe connection
// is closed immediately: the question is only whether something is bound.
func accepts(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}

	return conn.Close()
}

// TestStartAfterStopRefuses pins the single-use contract of grpc.Server.
// Start after a Stop used to set running=true and return nil while Serve
// returned ErrServerStopped, so the process advertised an address nothing
// served (and sicky.Run may register it).
func TestStartAfterStopRefuses(t *testing.T) {
	var startHooks int
	opts := &server.Options{Name: "grpc-start-after-stop"}
	opts.BeforeStart(func() error {
		startHooks++

		return nil
	})
	opts.AfterStart(func() error {
		startHooks++

		return nil
	})

	srv := New(opts, &Config{Network: "tcp", Address: "127.0.0.1:0"})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	addr := srv.Addr().String()
	if err := accepts(addr); err != nil {
		_ = srv.Stop()

		t.Fatalf("running server must accept on %s: %v", addr, err)
	}

	if !srv.Running() {
		t.Fatal("not running after Start")
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if srv.Running() {
		t.Fatal("still running after Stop")
	}

	// Only the second Start attempt is judged: the hooks above ran once for
	// the legitimate start.
	startHooks = 0

	err := srv.Start()
	if !errors.Is(err, grpc.ErrServerStopped) {
		t.Fatalf("Start after Stop = %v, want grpc.ErrServerStopped: reporting success while Serve rejects the stopped instance advertises a dead address", err)
	}

	if srv.Running() {
		t.Fatal("Running() must stay false after a refused restart")
	}

	if startHooks != 0 {
		t.Fatalf("start hooks ran %d times on a refused restart, want 0: a start that cannot bind must not report itself as started", startHooks)
	}

	// A refused Start must bind nothing and must not re-advertise: the
	// address stays the one that was stopped, and it accepts no connections.
	if got := srv.Addr().String(); got != addr {
		t.Fatalf("addr = %s after refused restart, want %s: Start bound a listener it cannot serve", got, addr)
	}

	if err := accepts(addr); err == nil {
		t.Fatalf("old address %s still accepts after Stop", addr)
	}
}

// TestStartStopLifecycle is the ordinary path: Start and Stop toggle
// Running and a repeated Stop stays a no-op. It is the regression guard for
// the stopped flag, which must only be set by a Stop that actually ran.
func TestStartStopLifecycle(t *testing.T) {
	srv := New(
		&server.Options{Name: "grpc-start-stop"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if !srv.Running() {
		t.Fatal("not running after Start")
	}

	if srv.Port() == 0 {
		_ = srv.Stop()

		t.Fatal("ephemeral port not assigned")
	}

	if err := accepts(srv.Addr().String()); err != nil {
		_ = srv.Stop()

		t.Fatalf("running server must accept: %v", err)
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if srv.Running() {
		t.Fatal("still running after Stop")
	}

	// The early "not running" return must not mark the instance spent, so a
	// double Stop stays harmless instead of changing state.
	if err := srv.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

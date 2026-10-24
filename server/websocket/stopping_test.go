/**
 * @file stopping_test.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package websocket

import (
	"net"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
)

// Stop is not instantaneous: it flips running=false, then drains for up to
// ShutdownTimeout (10s by default). Every other server implementation in this
// module guards that window with a stopping flag, so a Start landing inside it
// is refused rather than obeyed. Websocket did not, and the result was worse
// than a second listener: the new Start adds its serve and reaper goroutines to
// srv.wg, and the in-flight Stop is already sitting on wg.Wait() for that same
// group.
//
// So the concurrent Start did not merely race — it extended the drain it was
// supposed to be waiting out, by up to a second shutdown timeout.
//
// The drain window is held open by a blocking beforeStop hook. An earlier
// version held it open with a live, never-upgraded peer instead, which only
// works once fasthttp has accepted the connection: when the scheduler ran Stop
// to completion before the test reached Start, Start legitimately restarted the
// server and the test reported its own restart as a failure.
func TestStartDuringStopDrainIsRefused(t *testing.T) {
	draining := make(chan struct{})
	release := make(chan struct{})

	opts := &server.Options{Name: "drain"}
	// Stop sets running=false and stopping=true, then runs this hook before it
	// shuts the app down, so a signal from it means the window is definitely
	// open and the drain cannot finish until release is closed.
	opts.BeforeStop(func() error {
		close(draining)
		<-release

		return nil
	})

	srv := New(opts, &Config{Address: "127.0.0.1:0", ShutdownTimeout: 3})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	stopped := make(chan error, 1)
	go func() { stopped <- srv.Stop() }()

	// The hook runs mid-drain: running=false, stopping=true, wg not yet waited.
	<-draining

	started := srv.Start()

	// Release the drain regardless of what Start did, or Stop would never
	// return and a refusal bug would surface as a test timeout.
	close(release)

	if started != nil {
		t.Errorf("Start during the drain returned %v; it must refuse rather than "+
			"start a second listener inside the drain it is racing", started)
	}

	if srv.Running() {
		t.Error("Start during the drain started a second server")
	}

	select {
	case err := <-stopped:
		if err != nil {
			t.Logf("stop reported: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Stop never returned")
	}

	// The drain must not have outlived its own budget by waiting on
	// goroutines a concurrent Start added.
	if srv.Running() {
		t.Error("server is running after Stop returned")
	}
}

// The refusal above is only safe if a later Start, once the drain has
// finished, still works. A stopping flag that is never cleared would turn
// the server into a one-shot.
func TestStartWorksAgainAfterStopCompletes(t *testing.T) {
	srv := New(&server.Options{Name: "restart"}, &Config{Address: "127.0.0.1:0", ShutdownTimeout: 1})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("first start: %v", err)
	}

	first := srv.Addr().String()

	if err := srv.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start after a completed stop: %v", err)
	}

	defer func() {
		if err := srv.Stop(); err != nil {
			t.Errorf("final stop: %v", err)
		}
	}()

	if !srv.Running() {
		t.Fatal("server is not running after a successful restart")
	}

	second := srv.Addr().String()
	if second == "" {
		t.Fatal("restarted server has no address")
	}

	t.Logf("restart %s -> %s", first, second)
}

// A Stop issued against a server that is already draining must be a no-op,
// not a second drain over the same reaper channel — which would close an
// already-closed channel and panic.
func TestStopDuringStopIsSafe(t *testing.T) {
	srv := New(&server.Options{Name: "double"}, &Config{Address: "127.0.0.1:0", ShutdownTimeout: 2})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	conn := dialRaw(t, srv.Addr().String())
	defer conn.Close()

	first := make(chan error, 1)
	go func() { first <- srv.Stop() }()

	waitFor(t, func() bool { return !srv.Running() }, "first Stop never began its drain")

	second := srv.Stop()

	if second != nil {
		t.Errorf("second Stop returned %v, want nil: a server already draining is not running", second)
	}

	select {
	case <-first:
	case <-time.After(20 * time.Second):
		t.Fatal("first Stop never returned")
	}
}

func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatal(msg)
}

// dialRaw opens a plain TCP connection and leaves it open. It never completes
// a websocket upgrade, which is enough: the serve loop holds the socket and
// the drain has real work to do.
func dialRaw(t *testing.T, addr string) net.Conn {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}

	return conn
}

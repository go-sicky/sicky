/**
 * @file shutdown_test.go
 * @package protocol
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package protocol

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// addr is the transport's bound address.
func transportAddr(tr *HTTPTransport) string {
	return tr.ln.Addr().String()
}

// An SSE stream is an in-flight request for as long as the client stays
// connected, and handleSSE watches t.done — so closing done before Shutdown
// drains those cleanly, and that is why the ordinary case returns nil.
//
// What done cannot reach is a connection whose request never finished arriving.
// net/http holds it in StateActive, waiting for the rest of the request, with
// no handler running and therefore nothing watching done. Shutdown waits for
// it, times out, and this Stop discarded that with `_ = srv.Shutdown(ctx)`:
// a clean shutdown reported over a listener that was still open.
//
// The contract now: escalate to Close, and say so.
func TestStopForcesCloseWhenARequestWillNotFinish(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Send a request head and stop. The handler never runs, so closing done
	// does not reach this connection.
	conn, err := net.DialTimeout("tcp", transportAddr(tr), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer conn.Close()

	if _, err := conn.Write([]byte("POST /mcp/message HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n{\"a\":")); err != nil {
		t.Fatalf("write partial request: %v", err)
	}

	// Give the server time to accept it and start waiting for the body.
	time.Sleep(200 * time.Millisecond)

	// The peer deliberately stays connected: closing it would unblock the
	// server's read and let Shutdown succeed, which is the case the next
	// test covers. The 500ms goroutine net/http leaves behind after Close is
	// accounted for in internal/leakcheck, with the stdlib source quoted.
	start := time.Now()

	stopErr := tr.Stop()

	elapsed := time.Since(start)

	if stopErr == nil {
		t.Fatal("Stop returned nil while a request was still open: it reported a " +
			"shutdown that did not happen as one that did")
	}

	if !strings.Contains(stopErr.Error(), "forced close") {
		t.Errorf("error = %q, want it to name the forced close so the operator knows "+
			"the transport was cut rather than drained", stopErr)
	}

	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap the shutdown deadline so the cause survives", stopErr)
	}

	if elapsed > shutdownGrace+serveJoinTimeout+2*time.Second {
		t.Errorf("Stop took %v; the escalation must stay bounded", elapsed)
	}

	if after, derr := net.DialTimeout("tcp", transportAddr(tr), time.Second); derr == nil {
		_ = after.Close()
		t.Error("the transport is still accepting after Stop returned")
	}
}

// An SSE stream must still drain cleanly: the escalation is for requests that
// do not honor done, and must not fire on the ordinary case or every clean
// shutdown would look like a failure.
func TestStopDrainsAnSSEStreamCleanly(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	stream, _ := openSSE(t, http.DefaultClient, t.Context(), "http://"+transportAddr(tr))

	// Closed after Stop on purpose: closing it first would end the stream and
	// stop testing what this is about. Leaving it open unclosed is a real
	// leak, which is what the package's goleak check caught here first.
	defer stream.Close()

	if err := tr.Stop(); err != nil {
		t.Errorf("Stop with a live SSE stream returned %v, want nil: the handler "+
			"honors done, so this is an ordinary drain", err)
	}
}

// The escalation must not fire on a transport nobody connected to: that is
// the ordinary case, and reporting a failure for it would make every clean
// shutdown look like a problem.
func TestStopStaysQuietWhenNothingIsConnected(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := tr.Stop(); err != nil {
		t.Errorf("Stop with no clients returned %v, want nil", err)
	}
}

// A second Stop is a no-op, not a second escalation: Close on an already
// closed server returns an error, and reporting it would make every restart
// path look broken.
func TestStopIsIdempotent(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := tr.Stop(); err != nil {
		t.Fatalf("first stop: %v", err)
	}

	if err := tr.Stop(); err != nil {
		t.Errorf("second stop returned %v, want nil", err)
	}
}

// Stop before Start must not panic: there is no server and no serve goroutine.
func TestStopWithoutStart(t *testing.T) {
	if err := NewHTTPTransport("127.0.0.1:0").Stop(); err != nil {
		t.Errorf("Stop without Start returned %v, want nil", err)
	}
}

// When Stop returns, the serve goroutine must be gone. Otherwise the
// transport reports itself stopped while a goroutine is still inside Serve,
// which is what made the previous behavior hard to reason about.
func TestStopJoinsTheServeGoroutine(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := tr.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	done := make(chan struct{})

	go func() {
		tr.serveWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("the serve goroutine was still running when Stop returned")
	}
}

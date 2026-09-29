package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitGroupTimeoutDrained(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Done() // drained before anyone waits

	var drained atomic.Bool
	if !WaitGroupTimeout(&wg, time.Second, func() { drained.Store(true) }) {
		t.Fatal("a released group must count as drained")
	}

	// onDrained is guaranteed to have run before the waiter resumes, so
	// assert it directly: a caller that restarts the component on the
	// other side of this call must not see a stale draining flag.
	if !drained.Load() {
		t.Fatal("onDrained must run before the waiter resumes: a restart " +
			"after this call would observe draining=true and refuse")
	}
}

func TestWaitGroupTimeoutGivesUp(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	cleared := make(chan struct{})
	start := time.Now()
	ok := WaitGroupTimeout(&wg, 50*time.Millisecond, func() { close(cleared) })
	elapsed := time.Since(start)

	if ok {
		t.Fatal("an unreleased group must time out")
	}

	if elapsed > time.Second {
		t.Fatalf("timeout took %s, want ~50ms", elapsed)
	}

	// Releasing later must still fire onDrained (no lost update).
	wg.Done()

	select {
	case <-cleared:
	case <-time.After(time.Second):
		t.Fatal("onDrained did not run after the group drained")
	}
}

func TestWaitGroupTimeoutZeroWaits(t *testing.T) {
	var wg sync.WaitGroup

	if !WaitGroupTimeout(&wg, 0, nil) {
		t.Fatal("a zero timeout waits indefinitely and reports drained")
	}
}

// TestWaitTimeoutDrainsBeforeReturning pins the ordering that makes a
// stop-then-start cycle deterministic: by the time WaitTimeout reports
// drained, the caller's "still draining" flag is already cleared. The
// helper goroutine used to close(done) first and clear the flag second,
// which left a one-statement window where a restart got
// ErrStopTimeout from a component that was demonstrably idle.
//
// onDrained deliberately sleeps: the guarantee is that the waiter cannot
// resume until it has run, so a slow callback must not be observable as
// a still-set flag. That also makes the old ordering fail reliably
// instead of depending on a lucky goroutine schedule.
func TestWaitTimeoutDrainsBeforeReturning(t *testing.T) {
	var drained atomic.Bool

	ok := WaitTimeout(func() {}, time.Second, func() {
		time.Sleep(20 * time.Millisecond)
		drained.Store(true)
	})

	if !ok {
		t.Fatal("a released wait must report drained")
	}

	if !drained.Load() {
		t.Fatal("onDrained must run before the waiter resumes: draining " +
			"was still set on return, so a restart would see ErrStopTimeout")
	}
}

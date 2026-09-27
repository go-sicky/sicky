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

	// onDrained runs after Wait returns, so it may land just after the
	// caller resumes; poll briefly instead of asserting immediately.
	waitFor(t, drained.Load)
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

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition never became true")
}

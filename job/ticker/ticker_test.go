package ticker

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/utils"
)

func TestStopWaitsForLoop(t *testing.T) {
	var ticks atomic.Int64
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{Interval: 3600})
	if err := j.Add(&Task{Inteval: 1, Handler: func(time.Time, uint64) error {
		ticks.Add(1)

		return nil
	}}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Start-Stop-Start must not double-run the loop.
	for i := range 3 {
		if err := j.Start(); err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}

		if err := j.Stop(); err != nil {
			t.Fatalf("Stop %d: %v", i, err)
		}
	}
}

func TestRunWithTimeoutExpiry(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	hdl := &Task{
		ID:      uuid.New(),
		Timeout: 20 * time.Millisecond,
		Handler: func(time.Time, uint64) error {
			time.Sleep(500 * time.Millisecond)

			return nil
		},
	}

	start := time.Now()
	err := j.runWithTimeout(hdl, time.Now(), 1)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	if time.Since(start) > 400*time.Millisecond {
		t.Fatal("watchdog did not fire promptly")
	}

	plain := &Task{ID: uuid.New(), Handler: func(time.Time, uint64) error { return nil }}
	if err := j.runWithTimeout(plain, time.Now(), 1); err != nil {
		t.Fatalf("zero timeout must pass through, got %v", err)
	}
}

// TestStopTimesOutOnStuckLoop: a tick handler that never returns must not
// wedge the shutdown, and Start must stay refused while the old loop is
// still running - otherwise two loops would fire the same tasks.
func TestStopTimesOutOnStuckLoop(t *testing.T) {
	old := utils.StopTimeout
	utils.StopTimeout = 150 * time.Millisecond
	t.Cleanup(func() { utils.StopTimeout = old })

	release := make(chan struct{})
	entered := make(chan struct{}, 1)

	j := New(&job.Options{ID: uuid.New(), Name: "stuck"}, &Config{Interval: 1})
	if err := j.Add(&Task{Inteval: 1, Handler: func(time.Time, uint64) error {
		select {
		case entered <- struct{}{}:
		default:
		}

		<-release

		return nil
	}}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := j.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no tick within 5s")
	}

	if err := j.Stop(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Stop = %v, want ErrStopTimeout", err)
	}

	if err := j.Start(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Start while draining = %v, want ErrStopTimeout", err)
	}

	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := j.Start(); err == nil {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	if err := j.Stop(); err != nil {
		t.Fatalf("final stop: %v", err)
	}
}

// TestLoopKeepsItsOwnChannelsAcrossRestart replays the Stop/Start
// interleaving that used to hand a live loop somebody else's channels.
//
// Stop closes job.done, sets running=false and unlocks; a Start that
// lands in that window replaces job.done and job.ticker. A loop that
// re-read those fields on every iteration then selects on the *new*
// channels, so the close that was supposed to stop it is lost and the
// loop never exits. A loop that closed over its own channels keeps
// observing the closed one and exits promptly.
//
// The window between Stop's Unlock and its draining.Store is two
// statements wide and cannot be hit reliably from outside, so the test
// performs both critical sections by hand while the loop is parked
// inside a handler.
func TestLoopKeepsItsOwnChannelsAcrossRestart(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)

	j := New(&job.Options{ID: uuid.New(), Name: "channels"}, &Config{Interval: 1})
	if err := j.Add(&Task{Inteval: 1, Handler: func(time.Time, uint64) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release

		return nil
	}}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := j.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no tick within 5s")
	}

	// Stop's critical section, without the draining flag.
	j.Lock()
	close(j.done)
	j.ticker.Stop()
	j.running = false
	j.Unlock()

	// Start's critical section, landing in the window above.
	j.Lock()
	j.done = make(chan struct{})
	j.ticker = time.NewTicker(time.Duration(j.config.Interval) * time.Second)
	j.running = true
	j.Unlock()

	// Let the parked handler return so the loop reaches its select.
	close(release)

	// The loop must exit on the done channel it captured, not wait for
	// the replacement one this test installed.
	if !utils.WaitGroupTimeout(&j.wg, 2*time.Second, nil) {
		j.Lock()
		j.ticker.Stop()
		j.Unlock()

		t.Fatal("the loop did not exit after its own done channel was " +
			"closed: it re-read job.done and is now selecting on the " +
			"replacement channel, so the first Stop's close was lost")
	}

	// Clean up the state the test installed.
	j.Lock()
	j.ticker.Stop()
	j.running = false
	close(j.done)
	j.Unlock()
}

package ticker

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/job"
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

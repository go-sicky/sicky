package cron

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/utils"
)

func TestRunWithTimeoutExpiry(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	task := &Task{ID: uuid.New(), Timeout: 20 * time.Millisecond}
	wrapped := j.runWithTimeout(task, func() error {
		time.Sleep(500 * time.Millisecond)

		return nil
	})
	start := time.Now()
	err := wrapped()
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	if time.Since(start) > 400*time.Millisecond {
		t.Fatal("watchdog did not fire promptly")
	}
}

func TestRunWithTimeoutPassthrough(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	task := &Task{ID: uuid.New()}
	wrapped := j.runWithTimeout(task, func() error { return nil })
	if err := wrapped(); err != nil {
		t.Fatalf("zero timeout must pass through, got %v", err)
	}
}

// failingScheduler fails Shutdown so a Stop error can be reproduced.
type failingScheduler struct {
	gocron.Scheduler
}

func (failingScheduler) Shutdown() error { return errors.New("shutdown failed") }

// TestStopResetsRunningOnShutdownError: running used to stay true when
// Shutdown failed, so the next Start() returned early against a dead
// scheduler and no task ever fired again - while the metrics still
// reported the job as running.
func TestStopResetsRunningOnShutdownError(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	j.running = true
	j.scheduler = failingScheduler{}

	if err := j.Stop(); err == nil {
		t.Fatal("Stop must surface the shutdown error")
	}

	j.RLock()
	running := j.running
	j.RUnlock()

	if running {
		t.Fatal("running not reset after a failed Shutdown: the next Start would silently no-op")
	}
}

// TestStartStopRestart: a stopped cron must start again from scratch.
func TestStartStopRestart(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})

	if err := j.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := j.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if err := j.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}

	j.RLock()
	running, sched := j.running, j.scheduler
	j.RUnlock()

	if !running || sched == nil {
		t.Fatalf("after restart: running=%v scheduler=%v", running, sched)
	}

	if err := j.Stop(); err != nil {
		t.Fatalf("final stop: %v", err)
	}
}

// blockingScheduler lets a test hold Shutdown open the way a stuck job
// would.
type blockingScheduler struct {
	gocron.Scheduler
	release chan struct{}
}

func (b blockingScheduler) Shutdown() error {
	<-b.release

	return nil
}

// TestStopTimesOutOnBlockedShutdown: Shutdown waits for in-flight jobs,
// so it must be bounded - and it must not hold the job lock while
// waiting, or a concurrent Start would block on it.
func TestStopTimesOutOnBlockedShutdown(t *testing.T) {
	old := utils.StopTimeout
	utils.StopTimeout = 150 * time.Millisecond
	t.Cleanup(func() { utils.StopTimeout = old })

	j := New(&job.Options{ID: uuid.New(), Name: "blocked"}, &Config{})
	j.running = true
	sched := blockingScheduler{release: make(chan struct{})}
	j.scheduler = sched

	start := time.Now()
	if err := j.Stop(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Stop = %v, want ErrStopTimeout", err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Stop took %s, want ~150ms", elapsed)
	}

	// The scheduler is still draining: a second Start must not schedule a
	// duplicate copy of every job over it.
	if err := j.Start(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Start while draining = %v, want ErrStopTimeout", err)
	}

	close(sched.release)

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

package static

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/runner"
	"github.com/go-sicky/sicky/utils"
)

func newTestRunner(buffer, threads int) *Static {
	return New(
		&runner.Options{ID: uuid.New(), Name: "test", BufferSize: buffer, NThreads: threads},
		&Config{},
	)
}

func TestTryTaskBeforeStartRejected(t *testing.T) {
	r := newTestRunner(4, 1)
	if err := r.TryTask(&runner.Task{}, 0); !errors.Is(err, runner.ErrPoolFull) {
		t.Fatalf("TryTask before Start must return ErrPoolFull, got %v", err)
	}
}

func TestTryTaskFullReturnsErrPoolFull(t *testing.T) {
	r := newTestRunner(2, 1)
	// Block the single worker so the queue stays full.
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	r.options.Handler = func(*runner.Task) error {
		started <- struct{}{}
		<-release

		return nil
	}

	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = r.Stop() }()

	if err := r.TryTask(&runner.Task{}, time.Second); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	if err := r.TryTask(&runner.Task{}, time.Second); err != nil {
		t.Fatalf("second enqueue: %v", err)
	}

	<-started // worker picked up the first task, two remain queued
	if err := r.TryTask(&runner.Task{}, time.Second); err != nil {
		t.Fatalf("third enqueue: %v", err)
	}

	if err := r.TryTask(&runner.Task{}, 0); !errors.Is(err, runner.ErrPoolFull) {
		t.Fatalf("full queue must return ErrPoolFull, got %v", err)
	}

	if err := r.TryTask(&runner.Task{}, 20*time.Millisecond); !errors.Is(err, runner.ErrPoolFull) {
		t.Fatalf("full queue with timeout must return ErrPoolFull, got %v", err)
	}

	close(release)
}

func TestTryTaskAfterStopRejected(t *testing.T) {
	r := newTestRunner(4, 1)
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := r.TryTask(&runner.Task{}, time.Second); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := r.TryTask(&runner.Task{}, 0); !errors.Is(err, runner.ErrPoolFull) {
		t.Fatalf("TryTask after Stop must return ErrPoolFull, got %v", err)
	}
}

func TestLenTracksQueue(t *testing.T) {
	r := newTestRunner(4, 1)
	if got := r.Len(); got != 0 {
		t.Fatalf("fresh runner Len = %d, want 0", got)
	}

	release := make(chan struct{})
	started := make(chan struct{}, 4)
	r.options.Handler = func(*runner.Task) error {
		started <- struct{}{}
		<-release

		return nil
	}

	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	defer func() { _ = r.Stop() }()

	for i := range 3 {
		if err := r.TryTask(&runner.Task{}, time.Second); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	<-started // worker holds the first task, two remain queued
	if got := r.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}

	close(release)
}

func TestStartStopRestart(t *testing.T) {
	r := newTestRunner(4, 1)
	processed := make(chan string, 8)
	r.options.Handler = func(task *runner.Task) error {
		processed <- task.Data.(string)

		return nil
	}

	for i := range 3 {
		if err := r.Start(); err != nil {
			t.Fatalf("Start %d: %v", i, err)
		}

		tk := &runner.Task{Data: "v"}
		if err := r.TryTask(tk, time.Second); err != nil {
			t.Fatalf("TryTask %d: %v", i, err)
		}

		select {
		case got := <-processed:
			if got != "v" {
				t.Fatalf("task not processed: %q", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("task %d not processed", i)
		}

		if err := r.Stop(); err != nil {
			t.Fatalf("Stop %d: %v", i, err)
		}
	}
}

// shortenStopTimeout lowers the process-wide Stop bound for one test.
func shortenStopTimeout(t *testing.T, d time.Duration) {
	t.Helper()

	old := utils.StopTimeout
	utils.StopTimeout = d
	t.Cleanup(func() { utils.StopTimeout = old })
}

// TestStopTimesOutOnStuckWorker: a worker stuck in user code must not
// wedge the shutdown - and while it still runs, Start has to be refused
// (wg.Add during a pending Wait panics as a WaitGroup misuse, and the old
// batch would drain into the new pool).
func TestStopTimesOutOnStuckWorker(t *testing.T) {
	shortenStopTimeout(t, 150*time.Millisecond)

	release := make(chan struct{})
	entered := make(chan struct{}, 1)

	r := newTestRunner(4, 1)
	r.options.Handler = func(*runner.Task) error {
		select {
		case entered <- struct{}{}:
		default:
		}

		<-release

		return nil
	}

	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	r.TryTask(&runner.Task{}, time.Second)

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker never picked the task up")
	}

	start := time.Now()
	err := r.Stop()
	if !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Stop = %v, want ErrStopTimeout", err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Stop took %s, want ~150ms", elapsed)
	}

	if err := r.Start(); !errors.Is(err, utils.ErrStopTimeout) {
		t.Fatalf("Start while draining = %v, want ErrStopTimeout", err)
	}

	// Release the worker and wait for the runner to become startable.
	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := r.Start(); err == nil {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("final stop: %v", err)
	}
}

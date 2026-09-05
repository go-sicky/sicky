package static

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/runner"
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

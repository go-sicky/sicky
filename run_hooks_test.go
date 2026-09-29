package sicky

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/service"
)

// A failing BeforeStart hook must not tear down a healthy process.
// runErr used to do double duty — "return this to the caller" AND
// "fatal startup, skip the signal wait" — so the hook error made the
// `if runErr == nil` guard at the shutdown label false, Run skipped
// the signal wait and shut a successfully-started process down
// immediately, contradicting the contract in AGENTS §3.3.
//
// The distinguishing observable is that Run KEEPS SERVING: it must
// block in the signal wait until the context is canceled, not return
// straight away.
func TestRunHookFailureDoesNotAbortProcess(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx
	service.Clear()

	boom := errors.New("cache warm failed")
	BeforeStart(func(context.Context) error { return boom })

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Set(svc)

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}

	const hold = 400 * time.Millisecond

	go func() {
		time.Sleep(hold)
		cancel()
	}()

	start := time.Now()
	err := Run(cfg)
	elapsed := time.Since(start)

	// With the hook error folded into runErr this returns in
	// microseconds; the process never serves.
	if elapsed < hold/2 {
		t.Fatalf("Run returned after %v, want >= %v: the hook error aborted the process",
			elapsed, hold/2)
	}

	if !errors.Is(err, boom) {
		t.Fatalf("Run must still report the hook error, got %v", err)
	}

	if svc.starts != 1 {
		t.Fatalf("service starts = %d, want 1", svc.starts)
	}
}

// The hook error must be reported to the caller while the process keeps
// serving, so a returning Run must still wrap the failure.
func TestRunHookErrorReturnedToCaller(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx
	service.Clear()

	boom := errors.New("hook boom")
	BeforeStart(func(context.Context) error { return boom })
	AfterStart(func(context.Context) error { cancel(); return nil })

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}

	if err := Run(cfg); !errors.Is(err, boom) {
		t.Fatalf("Run must return the hook error, got %v", err)
	}
}

// Services must still start and stop normally when a hook fails —
// a hook error is not a startup failure.
func TestRunHookFailureStillRunsServiceLifecycle(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Clear()
	service.Set(svc)

	BeforeStart(func(context.Context) error { return errors.New("nope") })
	AfterStart(func(context.Context) error { cancel(); return nil })

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}
	_ = Run(cfg)

	if svc.starts != 1 {
		t.Fatalf("service starts = %d, want 1", svc.starts)
	}

	if svc.stops != 1 {
		t.Fatalf("service stops = %d, want 1", svc.stops)
	}
}

// BeforeStop/AfterStop failures are also hook errors, not shutdown
// errors: the rest of the shutdown must still complete.
func TestRunStopHookFailureIsReported(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Clear()
	service.Set(svc)

	AfterStart(func(context.Context) error { cancel(); return nil })

	stopBoom := errors.New("before stop boom")
	BeforeStop(func(context.Context) error { return stopBoom })

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}

	if err := Run(cfg); !errors.Is(err, stopBoom) {
		t.Fatalf("Run must return the stop hook error, got %v", err)
	}

	if svc.stops != 1 {
		t.Fatalf("service stops = %d, want 1", svc.stops)
	}
}

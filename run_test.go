package sicky

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

type fakeService struct {
	opts     *service.Options
	startErr []error
	starts   int
	stops    int
	mu       sync.Mutex
}

func (f *fakeService) Context() context.Context  { return context.Background() }
func (f *fakeService) Options() *service.Options { return f.opts }
func (f *fakeService) String() string            { return "fake" }
func (f *fakeService) Start() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++

	return f.startErr
}

func (f *fakeService) Stop() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++

	return nil
}

func (f *fakeService) Servers(...server.Server) []server.Server { return nil }
func (f *fakeService) Brokers(...broker.Broker) []broker.Broker { return nil }
func (f *fakeService) Jobs(...job.Job) []job.Job                { return nil }
func (f *fakeService) Registries(...registry.Registry) []registry.Registry {
	return nil
}

func (f *fakeService) Tracers(...tracer.Tracer) []tracer.Tracer { return nil }

// runTestGlobals snapshots process-global orchestration state so Run tests
// do not leak services, hooks, or options into each other.
func snapshotGlobals() func() {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	savedOpts := options
	savedInfra := MustInfra
	saved := [][]SickyWrapper{beforeStartWrappers, afterStartWrappers, beforeStopWrappers, afterStopWrappers, reloadWrappers}

	return func() {
		wrapperMu.Lock()
		defer wrapperMu.Unlock()
		options = savedOpts
		MustInfra = savedInfra
		beforeStartWrappers, afterStartWrappers, beforeStopWrappers, afterStopWrappers, reloadWrappers =
			saved[0], saved[1], saved[2], saved[3], saved[4]
		service.Clear()
	}
}

func testOptions() *Options {
	return &Options{
		AppName: "run-test",
		Version: "v0",
		Silence: true,
	}
}

func TestRunLifecycleOrder(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	var mu sync.Mutex
	var order []string
	mark := func(s string) SickyWrapper {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, s)

			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Clear()
	service.Set(svc)

	BeforeStart(mark("beforeStart"))
	AfterStart(func(context.Context) error {
		mu.Lock()
		order = append(order, "afterStart")
		mu.Unlock()
		cancel()

		return nil
	})
	BeforeStop(mark("beforeStop"))
	AfterStop(mark("afterStop"))

	cfg := &Config{Manager: &ManagerConfig{Enable: false}}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"beforeStart", "afterStart", "beforeStop", "afterStop"}
	idx := map[string]int{}
	for i, s := range order {
		idx[s] = i
	}

	for _, s := range want {
		if _, ok := idx[s]; !ok {
			t.Fatalf("missing hook %q in order %v", s, order)
		}
	}

	if idx["beforeStart"] >= idx["afterStart"] || idx["afterStart"] >= idx["beforeStop"] || idx["beforeStop"] >= idx["afterStop"] {
		t.Fatalf("hook order wrong: %v", order)
	}

	if svc.starts != 1 || svc.stops != 1 {
		t.Fatalf("service starts=%d stops=%d, want 1/1", svc.starts, svc.stops)
	}
}

func TestRunFailedServiceStopsOnce(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	boom := errors.New("boom")
	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}, startErr: []error{boom}}
	service.Clear()
	service.Set(svc)

	// Unblock the wait as soon as the failure path reaches shutdown.
	AfterStart(func(context.Context) error { cancel(); return nil })

	cfg := &Config{Manager: &ManagerConfig{Enable: false}}
	err := Run(cfg)
	if err == nil {
		t.Fatal("Run must return the service start error")
	}

	if !errors.Is(err, boom) {
		t.Fatalf("Run error must wrap the start error, got %v", err)
	}

	// One inline Stop after failed Start; the shutdown path must skip it.
	if svc.stops != 1 {
		t.Fatalf("failed service stopped %d times, want exactly 1", svc.stops)
	}
}

func TestRunCancelDuringWait(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	options = testOptions()
	options.Context = ctx
	service.Clear()

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	cfg := &Config{Manager: &ManagerConfig{Enable: false}}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

package sicky

import (
	"context"
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

// orderedService records the order in which its Start and Stop run. The
// shared fakeService only counts them, which cannot express a sequence.
type orderedService struct {
	opts   *service.Options
	events *[]string
	mu     *sync.Mutex
}

func (o *orderedService) Context() context.Context  { return context.Background() }
func (o *orderedService) Options() *service.Options { return o.opts }

func (o *orderedService) String() string { return o.opts.Name }

func (o *orderedService) Start() []error {
	o.mu.Lock()
	defer o.mu.Unlock()

	*o.events = append(*o.events, "start:"+o.opts.Name)

	return nil
}

func (o *orderedService) Stop() []error {
	o.mu.Lock()
	defer o.mu.Unlock()

	*o.events = append(*o.events, "stop:"+o.opts.Name)

	return nil
}

func (o *orderedService) Servers(...server.Server) []server.Server { return nil }

func (o *orderedService) Brokers(...broker.Broker) []broker.Broker { return nil }

func (o *orderedService) Jobs(...job.Job) []job.Job { return nil }

func (o *orderedService) Registries(...registry.Registry) []registry.Registry {
	return nil
}

func (o *orderedService) Tracers(...tracer.Tracer) []tracer.Tracer { return nil }

func newOrdered(name string, events *[]string, mu *sync.Mutex) *orderedService {
	return &orderedService{
		opts:   &service.Options{ID: uuid.New(), Name: name},
		events: events,
		mu:     mu,
	}
}

func TestRunStartsInRegistrationOrderAndStopsInReverse(t *testing.T) {
	restore := snapshotGlobals()
	t.Cleanup(restore)

	// Manager is tri-state: leaving the block nil means no manager, so the
	// test does not bind a port.
	managerApp = nil

	var (
		mu     sync.Mutex
		events []string
	)

	for _, name := range []string{"first", "second", "third"} {
		service.Set(newOrdered(name, &events, &mu))
	}

	// Run takes only a config and reads the package-level options
	// global, which snapshotGlobals already saves and restores.
	ctx, cancel := context.WithCancel(context.Background())
	options = testOptions()
	options.Context = ctx

	done := make(chan error, 1)

	go func() {
		// A config with no manager, broker, registry or infra binds no
		// port; Manager.Enable is new(false) for the same reason.
		done <- Run(&Config{Manager: &ManagerConfig{Enable: new(false)}})
	}()

	// Cancel once every service has started, so the stop order is observed
	// without racing the start loop.
	deadline := time.After(3 * time.Second)

	for {
		mu.Lock()
		starts := len(events)
		mu.Unlock()

		if starts == 3 {
			break
		}

		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("services did not start: events = %v", events)
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was canceled")
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{
		"start:first", "start:second", "start:third",
		"stop:third", "stop:second", "stop:first",
	}

	if len(events) != len(want) {
		t.Fatalf("lifecycle order = %v, want %v", events, want)
	}

	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("lifecycle order = %v, want %v: startup must follow registration order and teardown must mirror it", events, want)
		}
	}
}

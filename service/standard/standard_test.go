package standard

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

func TestStandardEmptyStartStop(t *testing.T) {
	service.Clear()
	svc := New(&service.Options{ID: uuid.New(), Name: "std-test"}, &Config{})
	if svc == nil {
		t.Fatal("New must succeed")
	}

	if errs := svc.Start(); len(errs) != 0 {
		t.Fatalf("empty Start: %v", errs)
	}

	if errs := svc.Stop(); len(errs) != 0 {
		t.Fatalf("empty Stop: %v", errs)
	}

	if service.Default() != svc {
		t.Fatal("first service must be Default")
	}

	service.Clear()
}

// Fakes satisfy the subordinate interfaces by embedding them: only the
// methods Start/Stop actually call need to exist.
type fakeServer struct{ server.Server }

func (fakeServer) Start() error { return nil }
func (fakeServer) Stop() error  { return nil }

type fakeBroker struct{ broker.Broker }

func (fakeBroker) Connect() error    { return nil }
func (fakeBroker) Disconnect() error { return nil }

type fakeJob struct{ job.Job }

func (fakeJob) Start() error { return nil }
func (fakeJob) Stop() error  { return nil }

type fakeRegistry struct{ registry.Registry }

func (fakeRegistry) Watch() error { return nil }
func (fakeRegistry) Stop() error  { return nil }

// fakeTracer is written out in full: the interface has a Tracer(...)
// method, so the embedding trick used above would collide with the
// embedded field of the same name.
type fakeTracer struct{}

func (fakeTracer) Context() context.Context           { return context.Background() }
func (fakeTracer) Options() *tracer.Options           { return &tracer.Options{} }
func (fakeTracer) String() string                     { return "fake" }
func (fakeTracer) ID() uuid.UUID                      { return uuid.Nil }
func (fakeTracer) Name() string                       { return "fake" }
func (fakeTracer) Start() error                       { return nil }
func (fakeTracer) Stop() error                        { return nil }
func (fakeTracer) Provider() *sdktrace.TracerProvider { return nil }
func (fakeTracer) Tracer(string) trace.Tracer         { return nil }

// TestConcurrentRegistrationAndLifecycle: the getters returned the live
// slices while Start/Stop walked them, so registering a subordinate
// mid-walk resized the very slice being iterated.
func TestConcurrentRegistrationAndLifecycle(t *testing.T) {
	svc := New(&service.Options{ID: uuid.New(), Name: "std-race"}, &Config{})
	if svc == nil {
		t.Fatal("New must succeed")
	}

	stop := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() {
		for range 200 {
			svc.Servers(fakeServer{})
			svc.Brokers(fakeBroker{})
			svc.Jobs(fakeJob{})
			svc.Registries(fakeRegistry{})
			svc.Tracers(fakeTracer{})

			// Getters hand out copies: mutating one must not reach in.
			servers := svc.Servers()
			if len(servers) > 0 {
				servers[0] = nil
			}
		}

		close(stop)
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = svc.Start()
				_ = svc.Stop()
			}
		}
	})

	wg.Wait()

	if len(svc.Servers()) == 0 {
		t.Fatal("registrations were lost")
	}
}

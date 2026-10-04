package sicky

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/tracer/stdout"
)

// recording reports whether a span started from tr is being recorded.
func recording(t *testing.T, tr tracer.Tracer) bool {
	t.Helper()

	_, span := tr.Tracer("probe").Start(context.Background(), "op")
	defer span.End()

	return span.IsRecording()
}

// The tracer was the only one of the four process-wide singletons that was
// stopped at shutdown but never cleared, while broker.Clear and
// registry.Clear run for exactly that reason. Because tracer.Set is
// first-wins, the stopped tracer stayed the process default and a second Run
// could never promote its replacement — so every server and client resolved a
// provider that had already been shut down and nothing was exported.
func TestStopTracerClearsTheDefaultSoTheNextRunCanRegister(t *testing.T) {
	tracer.Clear()

	t.Cleanup(tracer.Clear)

	first := stdout.New(&tracer.Options{ID: uuid.New(), Name: "first"}, nil)
	if first == nil {
		t.Fatal("first tracer was rejected")
	}

	if tracer.Default() != first {
		t.Fatal("first tracer must become the default")
	}

	if !recording(t, first) {
		t.Fatal("sanity: a fresh tracer must record")
	}

	if err := stopTracer(); err != nil {
		t.Fatalf("stopTracer: %v", err)
	}

	if recording(t, first) {
		t.Fatal("sanity: the provider must be down after Stop")
	}

	if got := tracer.Default(); got != nil {
		t.Fatalf("Default() = %v after shutdown, want nil: a stopped tracer left "+
			"behind blocks the next Run's tracer from ever becoming the default", got)
	}

	second := stdout.New(&tracer.Options{ID: uuid.New(), Name: "second"}, nil)
	if second == nil {
		t.Fatal("second tracer was rejected")
	}

	// Its batch span processor owns a goroutine; without this the test leaks
	// it and goleak fails the package.
	t.Cleanup(func() { _ = second.Stop() })

	if tracer.Default() != second {
		t.Error("the replacement tracer did not become the default: the next Run " +
			"would silently export nothing")
	}

	if !recording(t, tracer.Default()) {
		t.Error("the default must be a live provider after a restart")
	}
}

// Stopping when nothing was ever registered must not panic and must leave the
// registry clean, so a later Run starts from scratch.
func TestStopTracerWithNoDefaultIsANoOp(t *testing.T) {
	tracer.Clear()

	t.Cleanup(tracer.Clear)

	if err := stopTracer(); err != nil {
		t.Fatalf("stopTracer with no default = %v, want nil", err)
	}

	if tracer.Default() != nil {
		t.Errorf("Default() = %v, want nil", tracer.Default())
	}
}

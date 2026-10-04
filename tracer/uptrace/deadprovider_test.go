package uptrace

import (
	"context"
	"testing"

	"github.com/go-sicky/sicky/tracer"
)

// uptrace.ConfigureOpentelemetry returns early on a malformed DSN without
// installing a provider. New had already shut down whatever the global pointed
// at, so the type assertion on the global still succeeded and New handed back
// a tracer wrapping the corpse — non-nil, so the orchestrator set tcOk, logged
// "tracer initialized", and exported nothing for the life of the process.
//
// This only reproduces on a second New: on the first, the global is the OTel
// noop provider, which is not an *sdktrace.TracerProvider, so the assertion
// correctly failed and New returned nil. That is why it went unnoticed.
func TestNewWithMalformedDSNAfterAPreviousTracerReturnsNil(t *testing.T) {
	first := New(&tracer.Options{Name: "first"}, &Config{DSN: "https://tok@127.0.0.1:14317/1"})
	if first == nil {
		t.Skip("a valid DSN needs no reachable collector for provider setup; skipped")
	}

	t.Cleanup(func() { _ = first.Stop() })

	// One slash short: url.Parse reads it as a path, not a URL.
	second := New(&tracer.Options{Name: "second"}, &Config{DSN: "https//tok@127.0.0.1:14317/1"})
	if second == nil {
		return // the fix
	}

	t.Cleanup(func() { _ = second.Stop() })

	_, span := second.Tracer("probe").Start(context.Background(), "op")
	defer span.End()

	if span.IsRecording() {
		t.Fatal("sanity: the provider New returned must be the one it just shut down")
	}

	t.Fatal("New returned a tracer wrapping the provider it had just shut down. " +
		"sicky.Run would log \"tracer initialized\" and export nothing")
}

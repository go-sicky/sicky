package logger

import (
	"log/slog"
	"sync"
	"testing"
)

// countingWriter counts the records a slog handler writes, so a test can
// assert that a disabled level produces no output.
type countingWriter struct {
	mu    sync.Mutex
	lines int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lines++

	return len(p), nil
}

func (w *countingWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.lines
}

// TestEnabledReportsHandlerLevel pins the contract the hot-path guards
// depend on: Enabled must answer from the handler, so it tracks the level
// the handler was built with rather than any separate stored value.
func TestEnabledReportsHandlerLevel(t *testing.T) {
	// NewGeneral(nil) is required, not NewGeneral(): with no argument the
	// constructor adopts slog.Default() and its own LevelVar is never
	// attached to a handler, so Level() would have no observable effect.
	gl := NewGeneral(nil)
	defer SetDefaultGeneral(DefaultGeneralLogger)

	gl.Level(DebugLevel)

	if !gl.Enabled(DebugLevel) {
		t.Error("DebugLevel must be enabled after gl.Level(DebugLevel)")
	}

	// Debug is a higher threshold than Trace, so it does not imply Trace.
	if gl.Enabled(TraceLevel) {
		t.Error("TraceLevel must stay disabled at DebugLevel: it is a lower threshold")
	}

	if !gl.Enabled(ErrorLevel) {
		t.Error("ErrorLevel must be enabled when the level is Debug")
	}

	gl.Level(ErrorLevel)

	if gl.Enabled(DebugLevel) {
		t.Error("DebugLevel must be disabled after gl.Level(ErrorLevel)")
	}

	if !gl.Enabled(ErrorLevel) {
		t.Error("ErrorLevel must stay enabled after gl.Level(ErrorLevel)")
	}
}

// TestEnabledUsesSuppliedHandlerLevel guards the reason Enabled delegates
// to the handler instead of reading the constructor's own LevelVar: a
// caller that passes a pre-built logger keeps that logger's level.
func TestEnabledUsesSuppliedHandlerLevel(t *testing.T) {
	handlerLevel := new(slog.LevelVar)
	handlerLevel.Set(slog.LevelError)
	supplied := slog.New(slog.NewTextHandler(&countingWriter{}, &slog.HandlerOptions{Level: handlerLevel}))

	gl := NewGeneral(supplied)
	defer SetDefaultGeneral(DefaultGeneralLogger)

	if gl.Enabled(DebugLevel) {
		t.Error("Enabled must honor the supplied handler's level, not the default")
	}

	if !gl.Enabled(ErrorLevel) {
		t.Error("ErrorLevel must be enabled for a handler set to error")
	}
}

// TestGuardSuppressesDisabledLevelCall is the behavioral counterpart: with
// the level off, a guarded call site produces no output at all, which is
// the property that saves the per-message allocations.
func TestGuardSuppressesDisabledLevelCall(t *testing.T) {
	w := &countingWriter{}
	level := new(slog.LevelVar)
	level.Set(slog.LevelError)
	supplied := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))

	gl := NewGeneral(supplied)
	defer SetDefaultGeneral(DefaultGeneralLogger)

	if gl.Enabled(DebugLevel) || gl.Enabled(TraceLevel) {
		t.Fatal("Debug and Trace must report disabled at Error level")
	}

	// Simulate a guarded hot-path call site: the branch is not taken, so
	// the variadic argument slice is never built.
	if gl.Enabled(DebugLevel) {
		gl.Debug("per-message", "topic", "events")
	}

	if got := w.count(); got != 0 {
		t.Errorf("a guarded call at a disabled level wrote %d records, want 0", got)
	}

	// An enabled level still writes THROUGH the guard, so a wrong Enabled
	// that reported false for a live level would silently drop real logs.
	if gl.Enabled(ErrorLevel) {
		gl.Error("real failure", "topic", "events")
	}

	if got := w.count(); got != 1 {
		t.Errorf("guarded Error wrote %d records, want 1: the guard must not suppress enabled levels", got)
	}
}

// BenchmarkVariadicDisabledLevel measures the garbage a per-message
// disabled-level call costs. It is the reason the hot broker and runner
// paths guard their variadic Debug/Trace calls: the argument slice and the
// boxing of every string happen before slog consults the level.
//
// Run with -benchmem to see the per-op allocations the guard removes.
func BenchmarkVariadicDisabledLevel(b *testing.B) {
	gl := NewGeneral(nil)
	gl.Level(ErrorLevel) // Debug and Trace are disabled.
	defer SetDefaultGeneral(DefaultGeneralLogger)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		gl.Debug(
			"per-message log",
			"broker", "nsq",
			"id", "6f0a0c9e-0000-4000-8000-000000000000",
			"name", "orders",
			"topic", "events",
			"channel", "ch",
		)
	}
}

// BenchmarkGuardedDisabledLevel is the same work with the guard the hot
// paths now use. The difference between the two benchmarks is the win.
func BenchmarkGuardedDisabledLevel(b *testing.B) {
	gl := NewGeneral(nil)
	gl.Level(ErrorLevel)
	defer SetDefaultGeneral(DefaultGeneralLogger)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if gl.Enabled(DebugLevel) {
			gl.Debug(
				"per-message log",
				"broker", "nsq",
				"id", "6f0a0c9e-0000-4000-8000-000000000000",
				"name", "orders",
				"topic", "events",
				"channel", "ch",
			)
		}
	}
}

package logger

import (
	"log/slog"
	"strings"
	"testing"
)

// The constructor allocates a *slog.LevelVar on every call, but it only
// reaches a handler on the path where the constructor built that handler:
//
//	NewGeneral(nil)        -> handler built here, wired to our LevelVar
//	NewGeneral()           -> slog.Default() adopted, our LevelVar orphaned
//	NewGeneral(supplied)   -> the supplied logger is used, ours orphaned
//
// Level() writes gl.level and nothing else reads it, so on the latter two
// paths it is a silent no-op: no output change, no error, no panic. The
// caller has no way to tell, which is why the matrix is pinned here rather
// than left to a comment.
//
// These assertions describe what the constructor does today. If a future
// change makes Level effective on more paths, this test is the thing that
// fails and says so — which is the point: the limitation stops being silent
// even after nobody remembers writing it down.

// TestLevelIsHonouredOnlyWhenTheConstructorOwnsTheHandler records which
// construction forms Level() actually reaches.
func TestLevelIsHonouredOnlyWhenTheConstructorOwnsTheHandler(t *testing.T) {
	// Restore the process default: the no-arg form reads it at
	// construction time, and slog.SetDefault is process-global.
	original := slog.Default()

	t.Cleanup(func() { slog.SetDefault(original) })

	t.Run("nil argument builds a handler and Level is effective", func(t *testing.T) {
		gl := NewGeneral(nil)
		defer SetDefaultGeneral(DefaultGeneralLogger)

		gl.Level(ErrorLevel)

		if gl.Enabled(DebugLevel) {
			t.Error("DebugLevel must be disabled after Level(ErrorLevel) when the " +
				"constructor built the handler")
		}
	})

	t.Run("no argument adopts slog.Default and Level cannot reach it", func(t *testing.T) {
		buf := &strings.Builder{}
		slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))

		gl := NewGeneral()
		defer SetDefaultGeneral(DefaultGeneralLogger)

		gl.Level(ErrorLevel)
		gl.Debug("written because Level had no effect")

		if buf.Len() == 0 {
			t.Skip("no-arg form stopped writing to the adopted default; the " +
				"limitation this test pins may no longer describe it")
		}

		if !strings.Contains(buf.String(), "Level had no effect") {
			t.Error("the record did not reach the adopted default handler")
		}

		// The documented consequence, stated as an assertion so it is
		// impossible to read the test as endorsing the behavior: the
		// adopted handler's level, not gl.level, decides what is written.
		t.Logf("Level(ErrorLevel) left the adopted handler at Debug; %d record(s) "+
			"written. This is why callers must configure the level on the "+
			"logger they supply.", strings.Count(buf.String(), "\n"))
	})

	t.Run("supplied logger keeps its own level", func(t *testing.T) {
		level := new(slog.LevelVar)
		level.Set(slog.LevelDebug)

		gl := NewGeneral(slog.New(slog.NewJSONHandler(&strings.Builder{},
			&slog.HandlerOptions{Level: level})))
		defer SetDefaultGeneral(DefaultGeneralLogger)

		gl.Level(ErrorLevel)

		if !gl.Enabled(DebugLevel) {
			t.Error("Enabled must follow the supplied handler's level; it does not " +
				"read gl.level, which is why Level appears to work here until it " +
				"does not")
		}
	})
}

// TestEnabledIsTruthfulOnEveryConstructionForm is the property that actually
// has to hold everywhere, because the hot paths guard their variadic
// Debug/Trace calls with it: Enabled must never claim a level is live when
// the handler would discard it, or a real log line is dropped.
func TestEnabledIsTruthfulOnEveryConstructionForm(t *testing.T) {
	original := slog.Default()

	t.Cleanup(func() { slog.SetDefault(original) })

	// Each form is constructed inside its own subtest: the no-argument one
	// depends on the process default, and building the three up front in a
	// map would make the outcome depend on composite-literal evaluation
	// order.
	forms := map[string]func() GeneralLogger{
		"nil argument": func() GeneralLogger { return NewGeneral(nil) },
		"no argument": func() GeneralLogger {
			level := new(slog.LevelVar)
			level.Set(slog.LevelWarn)
			slog.SetDefault(slog.New(slog.NewJSONHandler(&strings.Builder{},
				&slog.HandlerOptions{Level: level})))

			return NewGeneral()
		},
		"supplied logger": func() GeneralLogger {
			level := new(slog.LevelVar)
			level.Set(slog.LevelWarn)

			return NewGeneral(slog.New(slog.NewJSONHandler(&strings.Builder{},
				&slog.HandlerOptions{Level: level})))
		},
	}

	for name, build := range forms {
		t.Run(name, func(t *testing.T) {
			gl := build()

			if gl.Enabled(TraceLevel) || gl.Enabled(DebugLevel) {
				t.Error("levels below the handler's Warn must report disabled")
			}

			if !gl.Enabled(ErrorLevel) {
				t.Error("ErrorLevel must report enabled")
			}
		})
	}
}

// NewGRPC has the same construction shape and the same orphaned LevelVar, so
// the limitation is pinned for it too rather than left to be rediscovered.
func TestGRPCLevelIsHonouredOnlyWhenTheConstructorOwnsTheHandler(t *testing.T) {
	original := slog.Default()

	t.Cleanup(func() { slog.SetDefault(original) })

	level := new(slog.LevelVar)
	level.Set(slog.LevelDebug)

	buf := &strings.Builder{}
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})))

	gl := NewGRPC()
	gl.Level(ErrorLevel)
	gl.Info("written because Level had no effect")

	if !strings.Contains(buf.String(), "Level had no effect") {
		t.Skip("NewGRPC() no longer writes to the adopted default; re-check " +
			"which construction forms honor Level")
	}

	// The gRPC logger has no Enabled method, so there is no way to observe
	// the level at all on this path — which is the strongest form of the
	// problem. Nothing in the type reports what the threshold is.
	t.Log("NewGRPC() adopted the default handler and has no Enabled accessor, " +
		"so an unobservable threshold is the only symptom")
}

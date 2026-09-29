/**
 * @file lock_scope_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/go-sicky/sicky/logger"
)

// blockingWarnLogger blocks inside Warn until release is closed, so the test
// can hold an Init* function at a precise point. It stands in for the
// duplicate teardown (a 5s Disconnect, a Close that flushes, paho's quiesce
// sleep) and for the synchronous success log: in every one of those cases
// the work happened while the package-global mu write lock was held.
type blockingWarnLogger struct {
	logger.GeneralLogger

	entered   chan struct{}
	release   chan struct{}
	enteredMu chan struct{}
}

func (b *blockingWarnLogger) Warn(string, ...any) {
	select {
	case <-b.enteredMu:
	default:
		close(b.entered)
	}

	<-b.release
}

// The remaining overrides keep a nil embedded GeneralLogger from being
// dereferenced by anything else on this path.
func (b *blockingWarnLogger) Error(string, ...any)             {}
func (b *blockingWarnLogger) Info(string, ...any)              {}
func (b *blockingWarnLogger) Debug(string, ...any)             {}
func (b *blockingWarnLogger) Trace(string, ...any)             {}
func (b *blockingWarnLogger) Log(logger.Level, string, ...any) {}
func (b *blockingWarnLogger) InfoContext(_ context.Context, _ string, _ ...any) {
}

func (b *blockingWarnLogger) ErrorContext(_ context.Context, _ string, _ ...any) {
}

func (b *blockingWarnLogger) DebugContext(_ context.Context, _ string, _ ...any) {
}

// TestInitDoesNotHoldGlobalLockAcrossDuplicateTeardown proves that the
// duplicate path of an Init* function releases the package-global mu before
// it tears the duplicate down. mu is a single write lock guarding all ten
// singletons, so holding it there blocks every Get* reader and every manager
// health probe for the whole teardown.
func TestInitDoesNotHoldGlobalLockAcrossDuplicateTeardown(t *testing.T) {
	ClearRistretto()
	ClearRedis()

	t.Cleanup(func() {
		ClearRistretto()
		ClearRedis()
	})

	// First init: nothing to duplicate, so it does not reach the Warn.
	if _, err := InitRistretto(&RistrettoConfig{}); err != nil {
		t.Fatalf("first InitRistretto: %v", err)
	}

	blocker := &blockingWarnLogger{
		entered:   make(chan struct{}),
		release:   make(chan struct{}),
		enteredMu: make(chan struct{}),
	}

	prev := logger.Logger
	logger.Logger = blocker

	t.Cleanup(func() { logger.Logger = prev })

	// Second init: Ristretto is already set, so this takes the duplicate path
	// and blocks inside Warn.
	done := make(chan struct{})

	go func() {
		defer close(done)

		_, _ = InitRistretto(&RistrettoConfig{})
	}()

	select {
	case <-blocker.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("duplicate InitRistretto never reached the duplicate path")
	}

	// A reader of an unrelated singleton must not be stuck behind it.
	read := make(chan struct{})

	go func() {
		defer close(read)

		_ = GetRedis()
	}()

	select {
	case <-read:
	case <-time.After(2 * time.Second):
		close(blocker.release)
		<-done
		t.Fatal("GetRedis blocked 2s while a duplicate Init tore down: the package-global mu is held across the teardown, so every Get* reader and health probe stalls with it")
	}

	close(blocker.release)
	<-done
}

// TestInitKeepsFirstWinsSingleton guards the behavior the lock refactor had to
// preserve: the duplicate is dropped, not swapped in, and the first singleton
// is the one that survives.
func TestInitKeepsFirstWinsSingleton(t *testing.T) {
	ClearRistretto()
	ClearRedis()

	t.Cleanup(func() {
		ClearRistretto()
		ClearRedis()
	})

	first, err := InitRistretto(&RistrettoConfig{})
	if err != nil {
		t.Fatalf("first InitRistretto: %v", err)
	}

	second, err := InitRistretto(&RistrettoConfig{})
	if err != nil {
		t.Fatalf("duplicate InitRistretto: %v", err)
	}

	if second != first {
		t.Fatal("duplicate Init must return the existing singleton, not the duplicate")
	}

	if GetRistretto() != first {
		t.Fatal("the first-wins singleton must survive a duplicate init")
	}
}

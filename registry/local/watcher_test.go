package local

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
)

// reloadCountingLogger counts the "watcher triggered" records, which the
// watcher emits exactly once per reload. The watcher's effect on the pool
// is not observable from this package (registry.PurgePool is a plain
// package-level function), so the per-reload log is the signal that lets
// the debounce be tested at all.
type reloadCountingLogger struct {
	logger.GeneralLogger

	mu    sync.Mutex
	count int
}

// DebugContext and ErrorContext are overridden because the nil embedded
// interface panics on any method the watcher actually calls, and the
// watcher logs on every event plus on every failed reload.
func (l *reloadCountingLogger) DebugContext(_ context.Context, _ string, _ ...any) {}

func (l *reloadCountingLogger) ErrorContext(_ context.Context, _ string, _ ...any) {}

func (l *reloadCountingLogger) InfoContext(_ context.Context, msg string, _ ...any) {
	if msg != "watcher triggered" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.count++
}

func (l *reloadCountingLogger) reloads() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.count
}

// waitForReloads waits for the reload count to reach at least want, and
// then reports the count observed after a quiet period. The quiet period
// matters: an assertion made as soon as the first reload lands would pass
// even with no debounce at all, because the extra reloads are still in
// flight.
func (l *reloadCountingLogger) waitForReloads(t *testing.T, want int) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if l.reloads() >= want {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Let any un-coalesced reload arrive before counting.
	time.Sleep(500 * time.Millisecond)

	return l.reloads()
}

// TestWatcherCoalescesBurstOfEvents pins the debounce: a burst of file
// writes in the watched directory must trigger exactly one reload. One
// os.WriteFile already emits CREATE and WRITE, so without coalescing even
// a single registration reloads twice, and a rolling restart of M peers
// reloads once per event per peer.
func TestWatcherCoalescesBurstOfEvents(t *testing.T) {
	dir := t.TempDir()

	counting := &reloadCountingLogger{}
	rg := New(&registry.Options{Logger: counting}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed on absolute tmp path")
	}

	rg.Watch()

	defer func() {
		rg.Stop()
		registry.Clear()
	}()

	// 20 peers registering back to back: at least 40 inotify events.
	for i := range 20 {
		if err := rg.Register(&registry.Instance{ID: uuid.New(), ServiceName: "svc"}); err != nil {
			t.Fatalf("register peer %d: %v", i, err)
		}
	}

	if got := counting.waitForReloads(t, 1); got != 1 {
		t.Fatalf("watcher reloaded %d times for one burst of 20 writes, want exactly 1: "+
			"each extra reload is a full directory scan plus a pool rebuild", got)
	}
}

// TestWatcherCoalescesRewriteOfOneFile is the single-file case, which is
// what a peer actually does on start: it writes one file, and the watcher
// must reload once rather than once per inotify event.
func TestWatcherCoalescesRewriteOfOneFile(t *testing.T) {
	dir := t.TempDir()

	counting := &reloadCountingLogger{}
	rg := New(&registry.Options{Logger: counting}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed on absolute tmp path")
	}

	rg.Watch()

	defer func() {
		rg.Stop()
		registry.Clear()
	}()

	if err := rg.Register(&registry.Instance{ID: uuid.New(), ServiceName: "svc"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if got := counting.waitForReloads(t, 1); got != 1 {
		t.Fatalf("watcher reloaded %d times for one file write, want exactly 1: "+
			"a single write emits CREATE and WRITE, which must coalesce", got)
	}

	// The coalesced pass must still publish the instance, or the
	// debounce would be "correct" only by never running.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pool := registry.GetPool(); pool != nil && pool.GetService("svc") != nil {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("pool has no svc entry after the coalesced reload: the debounce " +
		"must not swallow the reload")
}

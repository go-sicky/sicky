package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/registry"
)

// Deregistering an instance that is not there is a no-op in consul (the agent
// answers 200 for an unknown service id) and in redis (HDel on a missing field
// returns 0, nil). local returned a hard error instead, so the three
// implementations of one interface method disagreed about the same benign
// event — and sicky.Run joins a Deregister error into its return value, which
// turned an already-clean shutdown into a failed one. The register-failure
// path calls Deregister precisely to clean up after an instance that may never
// have been written.
func TestDeregisterAbsentIsNotAnError(t *testing.T) {
	rg := New(&registry.Options{}, &Config{RegistryFilePath: t.TempDir()})

	if err := rg.Deregister(uuid.New()); err != nil {
		t.Fatalf("deregistering an absent instance = %v, want nil: consul and "+
			"redis both treat this as success", err)
	}
}

// A real deregistration must still work, and must still remove the file.
func TestDeregisterRemovesTheFile(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})

	id := uuid.New()
	if err := rg.Register(&registry.Instance{ID: id, ServiceName: "svc"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	file := filepath.Join(dir, id.String()+".json")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file not written: %v", err)
	}

	if err := rg.Deregister(id); err != nil {
		t.Fatalf("deregister: %v", err)
	}

	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still present after deregister: %v", err)
	}
}

// Stop cancels rg.ctx for good — it is never re-derived. The watcher goroutine
// exits on ctx.Done() on its very first select, so a Watch() after Stop()
// rebuilt a watcher, logged "local registry watcher started", counted an ok
// and returned nil, while watching nothing at all: discovery silently died.
//
// Report the state instead of pretending a watcher is live. redis and consul
// are no-ops here for the same underlying reason (their contexts are not
// reused either) but never rebuild anything, so they never emit that false
// signal.
func TestWatchAfterStopDoesNotReportSuccess(t *testing.T) {
	rg := New(&registry.Options{}, &Config{RegistryFilePath: t.TempDir()})

	if err := rg.Watch(); err != nil {
		t.Fatalf("first watch: %v", err)
	}

	if err := rg.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	err := rg.Watch()
	if err == nil {
		t.Fatal("Watch after Stop returned nil: rg.ctx is canceled permanently " +
			"and the rebuilt watcher exits on its first select, so this reports a " +
			"watcher that is not running")
	}

	if !errors.Is(err, ErrWatchStopped) {
		t.Errorf("error = %v, want one wrapping ErrWatchStopped so a caller can tell "+
			"a stopped registry from a watcher that could not be created", err)
	}
}

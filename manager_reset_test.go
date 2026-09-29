package sicky

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/service"
)

// TestRunDropsTheManagerGlobalAfterShutdown pins that Run does not leave a
// stopped Manager behind in the package global.
//
// The global is only assigned when the manager starts, so a Run with the
// manager disabled skips that assignment. Without the reset in the shutdown
// sequence, a second Run therefore sees the first Run's already-stopped
// manager and advertises its dead address into every service's registry
// instance via serviceToRegistryInstance.
func TestRunDropsTheManagerGlobalAfterShutdown(t *testing.T) {
	restore := snapshotGlobals()
	t.Cleanup(restore)

	ctx, cancel := context.WithCancel(context.Background())
	options = testOptions()
	options.Context = ctx

	// Services start after the manager, so a service signaling its own
	// Start proves the manager is up without the test reading managerApp
	// while Run's goroutine is still writing it.
	var (
		mu      sync.Mutex
		started int
	)

	probe := &orderedService{
		opts:   &service.Options{ID: uuid.New(), Name: "c16-probe"},
		events: &[]string{},
		mu:     &mu,
	}
	service.Set(probe)

	defer service.Clear()

	// 127.0.0.1:0 is ephemeral, so the manager binds a real port without
	// colliding with anything else on the machine.
	cfg := &Config{Manager: &ManagerConfig{Enable: new(true), Address: "127.0.0.1:0"}}

	done := make(chan error, 1)

	go func() {
		done <- Run(cfg)
	}()

	// Wait on the manager's own listener through the access log-free path:
	// the manager assigns managerApp before it starts any service, so a
	// running service means the global is populated.
	waitFor(t, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()

		started = len(*probe.events)

		return started > 0
	}, "no service ever started, so the manager was never up and the test proves nothing")

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s")
	}

	// Safe to read only after Run has returned: until this point the global
	// was written by Run's own goroutine.
	if managerApp != nil {
		t.Fatalf("managerApp = %v after shutdown, want nil: a stopped manager must not be "+
			"advertised to the next Run", managerApp.Addr())
	}
}

// TestServiceToRegistryInstanceDropsStaleManagerAddress is the consequence
// the previous test prevents: with no live manager, a registry instance must
// carry no manager address at all.
func TestServiceToRegistryInstanceDropsStaleManagerAddress(t *testing.T) {
	restore := snapshotGlobals()
	t.Cleanup(restore)

	// serviceToRegistryInstance logs through the options global, so it has
	// to be populated even though this test starts no server.
	options = testOptions()

	// Run with the manager disabled leaves the global nil, which is exactly
	// the state a second Run sees after a first Run that used the manager.
	managerApp = nil

	ins := serviceToRegistryInstance(probeService("c16-dropped"))

	if ins.ManagerAddress != "" || ins.ManagerPort != 0 {
		t.Fatalf("registry instance advertises manager %q:%d, want empty: "+
			"no live manager means nothing to advertise", ins.ManagerAddress, ins.ManagerPort)
	}
}

// TestServiceToRegistryInstanceAdvertisesLiveManager is the positive half, so
// the test above cannot pass merely because the address is never populated.
//
// A Manager only knows its address once Start has bound the listener, so the
// manager is really started here and the expected values are read back from
// it rather than hardcoded.
func TestServiceToRegistryInstanceAdvertisesLiveManager(t *testing.T) {
	restore := snapshotGlobals()
	t.Cleanup(restore)

	options = testOptions()

	managerApp = NewManager(
		(&ManagerConfig{Enable: new(true), Address: "127.0.0.1:0"}).Ensure(),
		"c16-probe", "v0",
	)

	if err := managerApp.Start(); err != nil {
		t.Fatalf("manager start: %v", err)
	}

	wantAddr, wantPort := managerApp.Addr(), managerApp.Port()

	t.Cleanup(func() { _ = managerApp.Stop() })

	ins := serviceToRegistryInstance(probeService("c16-live"))

	if ins.ManagerAddress != wantAddr {
		t.Fatalf("manager address = %q, want %q: a live manager must be advertised",
			ins.ManagerAddress, wantAddr)
	}

	if ins.ManagerPort != wantPort {
		t.Fatalf("manager port = %d, want %d", ins.ManagerPort, wantPort)
	}
}

// probeService builds a registered service that records nothing, reusing the
// fake from run_order_test.go so there is only one service test double.
func probeService(name string) *orderedService {
	var (
		mu     sync.Mutex
		events []string
	)

	return &orderedService{
		opts:   &service.Options{ID: uuid.New(), Name: name},
		events: &events,
		mu:     &mu,
	}
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal(msg)
}

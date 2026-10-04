package registry

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

// InitPool is the one writer that swaps currentPool.Services without taking
// the pool's own mutex. Every other mutation (GetPool, PurgePool) and every
// exported Pool method takes poolLock *and* p.RWMutex, so the two-lock
// discipline is load-bearing in one direction and violated in the other.
//
// Reachable because InitPool is exported and returns the live pool: a caller
// holding it and calling its methods races a second Run()'s InitPool, and
// runMu only serializes Run bodies — it does not exclude pool readers.
func TestInitPoolDoesNotRaceDirectPoolMethod(t *testing.T) {
	InitPool()
	t.Cleanup(func() { InitPool() })

	p := InitPool() // the live pool, exactly as callers get it

	var wg sync.WaitGroup

	// Reader holding only p.RLock(), which is what every exported Pool method
	// does — they do not re-acquire poolLock.
	wg.Go(func() {
		for range 500 {
			_ = p.GetService("x")
		}
	})

	for range 500 {
		InitPool() // a concurrent second Run()
	}

	wg.Wait()
}

// RegisterService was the only pool write that stored the caller's pointer.
// RegisterInstance, GetService, GetInstance, GetInstances and PurgePool all
// deep-copy, and the file says so at each site — so a caller reusing its
// *Service or the *Instance values inside it mutated what every reader saw,
// concurrently with readers holding p.RLock().
func TestRegisterServiceCopiesTheCallersPointers(t *testing.T) {
	InitPool()
	t.Cleanup(func() { InitPool() })

	ins := &Instance{ID: uuid.New(), ServiceName: "svc", Type: "before"}

	RegisterService(&Service{
		Service:   "svc",
		Instances: map[uuid.UUID]*Instance{ins.ID: ins},
	})

	// The caller reuses its struct, as it is entitled to.
	ins.Type = "hijacked"

	got := GetInstances("svc")[ins.ID]
	if got == nil {
		t.Fatal("instance not registered")
	}

	if got.Type != "before" {
		t.Errorf("pool aliased the caller's Instance: Type = %q, want %q. Every "+
			"other pool write stores a deep copy", got.Type, "before")
	}

	// The same for the Service itself.
	svc := &Service{Service: "svc2"}
	RegisterService(svc)

	svc.Service = "renamed"

	if GetService("svc2") == nil {
		t.Error("pool aliased the caller's *Service: renaming it after " +
			"registration must not unregister it")
	}
}

package registry

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/utils"
)

func TestPoolConcurrentAccess(t *testing.T) {
	InitPool()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			id := uuid.New()
			ins := &Instance{ID: id, ServiceName: "svc"}
			RegisterInstance(ins)
			_ = GetInstance("svc", id)
			_ = GetInstances("svc")
			_ = GetService("svc")
			_ = GetPool()
			PurgePool([]*Instance{ins})
			UnregisterInstance("svc", id)
		})
	}

	wg.Wait()
}

func TestGetPoolSnapshotIsolation(t *testing.T) {
	InitPool()
	ins := &Instance{ID: uuid.New(), ServiceName: "svc"}
	RegisterService(&Service{Service: "svc", Instances: map[uuid.UUID]*Instance{ins.ID: ins}})
	snap := GetPool()
	if snap == nil {
		t.Fatal("nil snapshot")
	}

	PurgePool(nil)
	if len(snap.Services) != 1 {
		t.Fatalf("snapshot mutated by purge: %v", snap.Services)
	}
}

func TestPurgePoolKeepsPointer(t *testing.T) {
	InitPool()
	before := currentPool
	PurgePool([]*Instance{{ID: uuid.New(), ServiceName: "svc"}})
	if currentPool != before {
		t.Fatal("PurgePool swapped pool pointer, Notify channel not preserved")
	}
}

func TestNotifyChanStableAcrossPurge(t *testing.T) {
	InitPool()
	ch := NotifyChan()
	if ch == nil {
		t.Fatal("NotifyChan must be non-nil after InitPool")
	}

	PurgePool([]*Instance{{ID: uuid.New(), ServiceName: "svc"}})
	if NotifyChan() != ch {
		t.Fatal("NotifyChan must stay stable across PurgePool")
	}

	// Drain the notification from the purge above.
	select {
	case <-ch:
	default:
		t.Fatal("expected pool change notification")
	}

	PurgePool([]*Instance{{ID: uuid.New(), ServiceName: "svc2"}})
	select {
	case ev := <-ch:
		if !ev.Changed {
			t.Fatal("expected Changed event")
		}
	default:
		t.Fatal("expected pool change notification")
	}
}

// TestCloneInstanceNoJSONCycle guards the Topic->Instance back-reference
// removal: a cycle would make /services and every JSON registry backend
// fail to marshal.
func TestCloneInstanceNoJSONCycle(t *testing.T) {
	ins := &Instance{
		ID:          uuid.New(),
		ServiceName: "svc",
	}

	ins.Topics = map[string]*Topic{
		"events": {Name: "events", Type: "nats", Instance: ins},
	}

	dup := cloneInstance(ins)
	if dup.Topics["events"] == nil {
		t.Fatal("topic lost in clone")
	}

	raw, err := json.Marshal(dup)
	if err != nil {
		t.Fatalf("clone must marshal without cycle: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("empty marshal")
	}
}

// TestInitPoolIdempotentKeepsNotifyChan: InitPool used to rebuild the
// pool on every call, swapping the Notify channel out from under
// watchers that captured it via NotifyChan() - a second Run() would then
// leave them listening on a channel nobody writes to.
func TestInitPoolIdempotentKeepsNotifyChan(t *testing.T) {
	first := InitPool()
	notify := NotifyChan()
	if notify == nil {
		t.Fatal("NotifyChan must exist after InitPool")
	}

	// Seed state that the next Run() must not inherit.
	PurgePool([]*Instance{{ID: uuid.New(), ServiceName: "stale"}})
	if len(GetPool().Services) == 0 {
		t.Fatal("fixture: seeded service missing")
	}

	second := InitPool()
	if second != first {
		t.Fatal("InitPool must not replace the pool")
	}

	if NotifyChan() != notify {
		t.Fatal("NotifyChan must stay stable across InitPool")
	}

	if len(second.Services) != 0 {
		t.Fatalf("second Run inherited %d services, want a fresh pool", len(second.Services))
	}
}

// TestPurgePoolCopiesInstances: the pool stored caller pointers, so a
// caller reusing its Instance for the next registration raced every
// reader of GetPool/GetInstances.
func TestPurgePoolCopiesInstances(t *testing.T) {
	InitPool()

	ins := &Instance{ID: uuid.New(), ServiceName: "svc", Metadata: utils.NewMetadata()}
	ins.Metadata.Set("role", "api")

	PurgePool([]*Instance{ins})

	// The caller mutates its own instance after publishing it.
	ins.ServiceName = "renamed"
	ins.Metadata.Set("role", "attacker")

	got := GetInstance("svc", ins.ID)
	if got == nil {
		t.Fatal("instance missing from the pool")
	}

	if got.ServiceName != "svc" {
		t.Fatalf("service name = %q, want the published value", got.ServiceName)
	}

	if role, _ := got.Metadata.Get("role"); role != "api" {
		t.Fatalf("metadata = %v, want the published value", role)
	}
}

// TestRegisterInstanceIsTemporary documents the chosen semantics: a
// direct registration is replaced by the next discovery refresh.
func TestRegisterInstanceIsTemporary(t *testing.T) {
	InitPool()

	// RegisterInstance only attaches to a service discovery already
	// created, so seed the pool from a "backend" result first.
	backend := &Instance{ID: uuid.New(), ServiceName: "svc"}
	PurgePool([]*Instance{backend})

	ins := &Instance{ID: uuid.New(), ServiceName: "svc"}
	RegisterInstance(ins)

	if GetInstance("svc", ins.ID) == nil {
		t.Fatal("registered instance missing")
	}

	// The next refresh replaces the pool with the backend's result.
	PurgePool([]*Instance{backend})

	if GetInstance("svc", ins.ID) != nil {
		t.Fatal("a direct registration survived the purge: semantics changed")
	}

	if GetInstance("svc", backend.ID) == nil {
		t.Fatal("the backend instance must survive its own refresh")
	}
}

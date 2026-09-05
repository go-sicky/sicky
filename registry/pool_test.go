package registry

import (
	"sync"
	"testing"

	"github.com/google/uuid"
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

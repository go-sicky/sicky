package registry

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestPoolConcurrentAccess(t *testing.T) {
	InitPool()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uuid.New()
			ins := &Instance{ID: id, ServiceName: "svc"}
			RegisterInstance(ins)
			_ = GetInstance("svc", id)
			_ = GetInstances("svc")
			_ = GetService("svc")
			_ = GetPool()
			PurgePool([]*Instance{ins})
			UnregisterInstance("svc", id)
		}()
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
	if GetService("svc") == nil {
		t.Fatal("fresh instance missing after purge")
	}
}

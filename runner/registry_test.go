package runner

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type fakeRunner struct{ id uuid.UUID }

func (f *fakeRunner) Context() context.Context { return context.Background() }
func (f *fakeRunner) Options() *Options        { return nil }
func (f *fakeRunner) String() string           { return "fake" }
func (f *fakeRunner) Name() string             { return "fake" }
func (f *fakeRunner) ID() uuid.UUID            { return f.id }
func (f *fakeRunner) Start() error             { return nil }
func (f *fakeRunner) Stop() error              { return nil }
func (f *fakeRunner) Task(*Task)               {}

func TestRegistryConcurrentAccess(t *testing.T) {
	Clear()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := &fakeRunner{id: uuid.New()}
			Set(r)
			_ = Get(r.id)
			_ = Default()
			_ = Runners()
		}()
	}
	wg.Wait()
}

func TestRegistryDefaultAndClear(t *testing.T) {
	Clear()
	if Default() != nil {
		t.Fatal("Default must be nil after Clear")
	}
	a := &fakeRunner{id: uuid.New()}
	b := &fakeRunner{id: uuid.New()}
	Set(a, b)
	if Default() != a {
		t.Fatal("first registered must be Default")
	}
	if len(Runners()) != 2 {
		t.Fatalf("Runners = %d, want 2", len(Runners()))
	}
	Clear()
	if len(Runners()) != 0 || Default() != nil {
		t.Fatal("Clear must reset registry")
	}
}

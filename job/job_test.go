package job

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type fakeJob struct{ id uuid.UUID }

func (f *fakeJob) Context() context.Context { return context.Background() }
func (f *fakeJob) Options() *Options        { return nil }
func (f *fakeJob) String() string           { return "fake" }
func (f *fakeJob) Name() string             { return "fake" }
func (f *fakeJob) ID() uuid.UUID            { return f.id }
func (f *fakeJob) Start() error             { return nil }
func (f *fakeJob) Stop() error              { return nil }

func TestRegistryConcurrentAccess(t *testing.T) {
	Clear()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j := &fakeJob{id: uuid.New()}
			Set(j)
			_ = Get(j.id)
			_ = Default()
			_ = Jobs()
		}()
	}
	wg.Wait()
}

func TestRegistryDefaultAndClear(t *testing.T) {
	Clear()
	if Default() != nil {
		t.Fatal("Default must be nil after Clear")
	}
	a := &fakeJob{id: uuid.New()}
	b := &fakeJob{id: uuid.New()}
	Set(a, b)
	if Default() != a {
		t.Fatal("first registered must be Default")
	}
	if len(Jobs()) != 2 {
		t.Fatalf("Jobs = %d, want 2", len(Jobs()))
	}
	Clear()
	if len(Jobs()) != 0 || Default() != nil {
		t.Fatal("Clear must reset registry")
	}
}

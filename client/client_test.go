package client

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type fakeClient struct{ id uuid.UUID }

func (f *fakeClient) Context() context.Context { return context.Background() }
func (f *fakeClient) Options() *Options        { return nil }
func (f *fakeClient) Connect() error           { return nil }
func (f *fakeClient) Disconnect() error        { return nil }
func (f *fakeClient) Call() error              { return nil }
func (f *fakeClient) String() string           { return "fake" }
func (f *fakeClient) Name() string             { return "fake" }
func (f *fakeClient) ID() uuid.UUID            { return f.id }

func TestRegistryConcurrentAccess(t *testing.T) {
	Clear()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			c := &fakeClient{id: uuid.New()}
			Set(c)
			_ = Get(c.id)
			_ = Default()
			_ = Clients()
		})
	}

	wg.Wait()
}

func TestRegistryDefaultAndClear(t *testing.T) {
	Clear()
	if Default() != nil {
		t.Fatal("Default must be nil after Clear")
	}

	a := &fakeClient{id: uuid.New()}
	b := &fakeClient{id: uuid.New()}
	Set(a, b)
	if Default() != a {
		t.Fatal("first registered must be Default")
	}

	if len(Clients()) != 2 {
		t.Fatalf("Clients = %d, want 2", len(Clients()))
	}

	Clear()
	if len(Clients()) != 0 || Default() != nil {
		t.Fatal("Clear must reset registry")
	}
}

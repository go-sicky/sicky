package server

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/utils"
)

type fakeServer struct{ id uuid.UUID }

func (f *fakeServer) Context() context.Context { return context.Background() }
func (f *fakeServer) Options() *Options        { return nil }
func (f *fakeServer) String() string           { return "fake" }
func (f *fakeServer) ID() uuid.UUID            { return f.id }
func (f *fakeServer) Name() string             { return "fake" }
func (f *fakeServer) Start() error             { return nil }
func (f *fakeServer) Stop() error              { return nil }
func (f *fakeServer) Running() bool            { return false }
func (f *fakeServer) Addr() net.Addr           { return nil }
func (f *fakeServer) IP() net.IP               { return nil }
func (f *fakeServer) Port() int                { return 0 }
func (f *fakeServer) AdvertiseAddr() net.Addr  { return nil }
func (f *fakeServer) AdvertiseIP() net.IP      { return nil }
func (f *fakeServer) AdvertisePort() int       { return 0 }
func (f *fakeServer) Metadata() utils.Metadata { return nil }

func TestRegistryConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			s := &fakeServer{id: uuid.New()}
			Set(s)
			_ = Get(s.id)
			_ = Default()
			_ = Servers()
		})
	}

	wg.Wait()
}

func TestRegistryDefaultAndClear(t *testing.T) {
	Clear()

	a := &fakeServer{id: uuid.New()}
	b := &fakeServer{id: uuid.New()}
	Set(a, b)

	if Default() != Server(a) {
		t.Fatal("first registered server should be the default")
	}

	snap := Servers()
	if len(snap) != 2 {
		t.Fatalf("Servers() = %d entries, want 2", len(snap))
	}

	delete(snap, a.id)
	if len(Servers()) != 2 {
		t.Fatal("Servers() must return a copy")
	}

	Clear()
	if Default() != nil || len(Servers()) != 0 {
		t.Fatal("Clear() must reset the registry")
	}
}

package server

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
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
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &fakeServer{id: uuid.New()}
			Set(s)
			_ = Get(s.id)
		}()
	}
	wg.Wait()
}

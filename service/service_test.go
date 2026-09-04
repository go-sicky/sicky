package service

import (
	"context"
	"sync"
	"testing"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
)

type fakeService struct {
	opts *Options
}

func (f *fakeService) Context() context.Context { return context.Background() }
func (f *fakeService) Options() *Options        { return f.opts }
func (f *fakeService) String() string           { return "fake" }
func (f *fakeService) Start() []error           { return nil }
func (f *fakeService) Stop() []error            { return nil }
func (f *fakeService) Servers(...server.Server) []server.Server {
	return nil
}
func (f *fakeService) Brokers(...broker.Broker) []broker.Broker { return nil }
func (f *fakeService) Jobs(...job.Job) []job.Job                { return nil }
func (f *fakeService) Registries(...registry.Registry) []registry.Registry {
	return nil
}
func (f *fakeService) Tracers(...tracer.Tracer) []tracer.Tracer { return nil }

func TestRegistryConcurrentAccess(t *testing.T) {
	Clear()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &fakeService{opts: &Options{ID: uuid.New(), Name: "fake"}}
			Set(s)
			_ = Get(s.opts.ID)
			_ = Default()
			_ = Services()
		}()
	}
	wg.Wait()
	Clear()
}

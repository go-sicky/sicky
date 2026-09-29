package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRegistry satisfies Registry so the package helpers can be
// exercised without a backend. Every method records that it was
// reached, which is how the delegation tests tell "the helper forwarded"
// apart from "the helper returned early".
type fakeRegistry struct {
	id        uuid.UUID
	instances []*Instance
	loadErr   error
	loadHits  int
}

func (f *fakeRegistry) Context() context.Context { return context.Background() }
func (f *fakeRegistry) Options() *Options        { return nil }
func (f *fakeRegistry) String() string           { return "fake" }
func (f *fakeRegistry) Name() string             { return "fake" }
func (f *fakeRegistry) ID() uuid.UUID            { return f.id }

func (f *fakeRegistry) Register(ins *Instance) error    { return nil }
func (f *fakeRegistry) Deregister(id uuid.UUID) error   { return nil }
func (f *fakeRegistry) CheckInstance(id uuid.UUID) bool { return false }
func (f *fakeRegistry) Watch() error                    { return nil }
func (f *fakeRegistry) Stop() error                     { return nil }

func (f *fakeRegistry) Load() ([]*Instance, error) {
	f.loadHits++

	return f.instances, f.loadErr
}

// Load must not answer (nil, nil) when no registry is configured:
// that is indistinguishable from "a reachable registry that currently
// holds no instances", so a misconfigured deployment discovers nothing
// and cannot tell why.
func TestLoadReportsUninitialized(t *testing.T) {
	Clear()

	ins, err := Load()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Load without a registry: %v, want %v", err, ErrNotInitialized)
	}

	if ins != nil {
		t.Fatalf("Load without a registry must return no instances, got %d", len(ins))
	}
}

// CheckInstance's signature predates the sentinels and cannot report an
// error. false is the conservative answer either way, so it stays.
func TestCheckInstanceStaysFalseSafe(t *testing.T) {
	Clear()

	if CheckInstance(uuid.New()) {
		t.Fatal("CheckInstance without a registry must be false")
	}
}

// Once a registry is registered Load must delegate instead of
// reporting the sentinel.
func TestLoadDelegatesWhenInitialized(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	want := []*Instance{{ID: uuid.New(), ServiceName: "svc"}}
	rg := &fakeRegistry{id: uuid.New(), instances: want}
	Set(rg)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load with a registry registered: %v, want nil", err)
	}

	if rg.loadHits != 1 {
		t.Fatalf("the helper did not delegate: the registry's Load was called %d times, want 1", rg.loadHits)
	}

	if len(got) != 1 || got[0].ServiceName != "svc" {
		t.Fatalf("Load returned %+v, want the registry's own instances", got)
	}
}

// A backend error must reach the caller untouched, not be flattened
// into the not-initialized sentinel.
func TestLoadPropagatesBackendError(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	boom := errors.New("backend unreachable")
	Set(&fakeRegistry{id: uuid.New(), loadErr: boom})

	if _, err := Load(); !errors.Is(err, boom) {
		t.Fatalf("Load with a failing backend: %v, want %v", err, boom)
	}
}

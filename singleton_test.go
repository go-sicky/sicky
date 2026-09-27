package sicky

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/registry"
)

// fakeBroker is a minimal broker.Broker for lifecycle tests.
type fakeBroker struct{}

func (f *fakeBroker) Context() context.Context               { return context.Background() }
func (f *fakeBroker) Options() *broker.Options               { return &broker.Options{} }
func (f *fakeBroker) String() string                         { return "fake" }
func (f *fakeBroker) ID() uuid.UUID                          { return uuid.Nil }
func (f *fakeBroker) Name() string                           { return "fake" }
func (f *fakeBroker) Connect() error                         { return nil }
func (f *fakeBroker) Disconnect() error                      { return nil }
func (f *fakeBroker) Publish(string, *broker.Message) error  { return nil }
func (f *fakeBroker) Subscribe(string, broker.Handler) error { return nil }
func (f *fakeBroker) Unsubscribe(string) error               { return nil }

// fakeRegistry is a minimal registry.Registry for lifecycle tests.
type fakeRegistry struct{}

func (f *fakeRegistry) Context() context.Context            { return context.Background() }
func (f *fakeRegistry) Options() *registry.Options          { return &registry.Options{} }
func (f *fakeRegistry) String() string                      { return "fake" }
func (f *fakeRegistry) ID() uuid.UUID                       { return uuid.Nil }
func (f *fakeRegistry) Name() string                        { return "fake" }
func (f *fakeRegistry) Register(*registry.Instance) error   { return nil }
func (f *fakeRegistry) Deregister(uuid.UUID) error          { return nil }
func (f *fakeRegistry) CheckInstance(uuid.UUID) bool        { return false }
func (f *fakeRegistry) Load() ([]*registry.Instance, error) { return nil, nil }
func (f *fakeRegistry) Watch() error                        { return nil }
func (f *fakeRegistry) Stop() error                         { return nil }

// TestRunRespectsMustFlags: the must-have checks only work if Run()
// re-derives the flags. Init() runs once, while a second Run() finds the
// flags already cleared to false by the first run's startup checks - the
// requirement would then be skipped silently.
func TestRunRespectsMustFlags(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	// What the first Run leaves behind: the startup checks set it false.
	MustBroker = false

	options = testOptions()
	options.Context = t.Context()
	options.MustBroker = true // Init() would have set this once

	cfg := &Config{Manager: &ManagerConfig{Enable: false}}
	err := Run(cfg)
	if !errors.Is(err, ErrBrokerNotInitialized) {
		t.Fatalf("Run with MustBroker and no broker = %v, want ErrBrokerNotInitialized", err)
	}
}

// TestRunClearsSingletons: shutdown must drop the broker/registry
// singletons. Otherwise the package helpers (broker.Publish,
// registry.Register) keep pointing at an already-disconnected instance
// for the rest of the process - and a second Run() builds fresh ones that
// nothing references.
func TestRunClearsSingletons(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	broker.Set(&fakeBroker{})
	registry.Set(&fakeRegistry{})

	// Run() blocks in the signal wait once startup succeeds: cancel as
	// soon as the hooks confirm the start so shutdown (and the Clear
	// assertions) can run.
	AfterStart(func(context.Context) error {
		cancel()

		return nil
	})

	cfg := &Config{Manager: &ManagerConfig{Enable: false}}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if broker.Default() != nil {
		t.Fatal("broker singleton survived Run: a second Run would publish into a disconnected instance")
	}

	if registry.Default() != nil {
		t.Fatal("registry singleton survived Run: a second Run would register into a stopped instance")
	}
}

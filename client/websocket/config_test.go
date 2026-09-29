/**
 * @file config_test.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 09/30/2026
 */

package websocket

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
)

// TestConfigEnsureIsNilSafe pins the nil-receiver contract every top-level
// Config in this framework shares: Ensure on a nil pointer must return a
// usable value, and on a non-nil pointer it must return the receiver so a
// caller can chain c.Ensure().Foo.
func TestConfigEnsureIsNilSafe(t *testing.T) {
	var nilCfg *Config

	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return a usable config, not nil")
	}

	cfg := &Config{}
	if got := cfg.Ensure(); got != cfg {
		t.Fatalf("Ensure returned %p, want the receiver %p", got, cfg)
	}

	if DefaultConfig() == nil {
		t.Fatal("DefaultConfig must not return nil")
	}
}

// TestNewAcceptsNilOptionsAndConfig covers the constructor's own
// nil-tolerance. New calls opts.Ensure() and cfg.Ensure() before touching
// either, so a caller that passes nothing must get a working client rather
// than a panic: a framework constructor that dereferences its arguments
// before normalizing them turns every optional-argument call site into a
// crash.
func TestNewAcceptsNilOptionsAndConfig(t *testing.T) {
	client.Clear()
	t.Cleanup(client.Clear)

	clt := New(nil, nil)
	if clt == nil {
		t.Fatal("New(nil, nil) must return a client")
	}

	if clt.Options() == nil {
		t.Fatal("Options must be populated by New even for nil input")
	}

	if clt.Context() == nil {
		t.Fatal("Context must be populated by New even for nil input")
	}
}

// TestNewRegistersTheClient pins the self-registration contract: the
// client abstraction is populated by its own New so the framework can
// reach it without an explicit Set. The first registration also becomes
// the package default.
func TestNewRegistersTheClient(t *testing.T) {
	client.Clear()
	t.Cleanup(client.Clear)

	clt := New(&client.Options{ID: uuid.New(), Name: "ws-test"}, &Config{})
	if clt == nil {
		t.Fatal("New must return a client")
	}

	if got := client.Get(clt.Options().ID); got == nil {
		t.Fatal("New must register the client so client.Get can find it")
	}

	if client.Default() == nil {
		t.Fatal("the first registered client must become the default")
	}
}

// TestConnectAndDisconnectAreNoOps documents the current transport state
// rather than asserting a bug: the websocket client has no dial path yet,
// so both methods are stubs that succeed unconditionally. If a transport
// is ever added, this test is the reminder to replace it with one that
// actually exercises the dial and its failure mode.
func TestConnectAndDisconnectAreNoOps(t *testing.T) {
	client.Clear()
	t.Cleanup(client.Clear)

	clt := New(&client.Options{ID: uuid.New(), Name: "ws-stub"}, &Config{})
	if clt == nil {
		t.Fatal("New must return a client")
	}

	if err := clt.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := clt.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

// TestNewHonorsCallerSuppliedContext guards against New silently replacing
// a caller's context with a background one, which would detach the client
// from the shutdown and signal plumbing the caller wired up.
func TestNewHonorsCallerSuppliedContext(t *testing.T) {
	client.Clear()
	t.Cleanup(client.Clear)

	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")

	clt := New(&client.Options{Context: ctx}, &Config{})
	if clt == nil {
		t.Fatal("New must return a client")
	}

	if got := clt.Context().Value(ctxKey{}); got != "sentinel" {
		t.Fatalf("Context value = %v, want the caller-supplied context to be preserved", got)
	}
}

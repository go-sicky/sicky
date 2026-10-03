package http

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
)

func TestClientLifecycle(t *testing.T) {
	clt := New(&client.Options{ID: uuid.New(), Name: "http-test"}, &Config{})
	if clt == nil {
		t.Fatal("New must succeed")
	}

	if err := clt.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Call has no target in its signature, so it cannot carry a request and
	// must report that rather than a false success.
	if err := clt.Call(); !errors.Is(err, client.ErrClientNotImplemented) {
		t.Fatalf("Call = %v, want ErrClientNotImplemented", err)
	}

	if err := clt.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

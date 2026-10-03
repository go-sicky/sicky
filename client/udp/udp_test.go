package udp

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
)

func TestClientResolve(t *testing.T) {
	clt, err := New(&client.Options{ID: uuid.New(), Name: "udp-test"}, &Config{Addr: "127.0.0.1:9980"})
	if err != nil || clt == nil {
		t.Fatalf("New valid addr: %v", err)
	}

	// Call has no target in its signature, so it cannot carry a request and
	// must report that rather than a false success.
	if err := clt.Call(); !errors.Is(err, client.ErrClientNotImplemented) {
		t.Fatalf("Call = %v, want ErrClientNotImplemented", err)
	}

	if _, err := New(&client.Options{ID: uuid.New(), Name: "udp-test"}, &Config{Addr: "://bad"}); err == nil {
		t.Fatal("New bad addr must fail")
	}
}

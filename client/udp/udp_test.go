package udp

import (
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
)

func TestClientResolve(t *testing.T) {
	clt, err := New(&client.Options{ID: uuid.New(), Name: "udp-test"}, &Config{Addr: "127.0.0.1:9980"})
	if err != nil || clt == nil {
		t.Fatalf("New valid addr: %v", err)
	}

	if err := clt.Call(); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if _, err := New(&client.Options{ID: uuid.New(), Name: "udp-test"}, &Config{Addr: "://bad"}); err == nil {
		t.Fatal("New bad addr must fail")
	}
}

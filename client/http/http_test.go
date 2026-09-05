package http

import (
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

	if err := clt.Call(); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if err := clt.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

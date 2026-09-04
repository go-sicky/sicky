package fiber

import (
	"errors"
	"testing"

	"github.com/go-sicky/sicky/server"
)

func TestFiberStartStopRestart(t *testing.T) {
	srv := New(
		&server.Options{Name: "fiber-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !srv.Running() {
		t.Fatal("not running after Start")
	}
	if srv.Port() == 0 {
		t.Fatal("ephemeral port not assigned")
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if srv.Running() {
		t.Fatal("still running after Stop")
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if err := srv.Stop(); err != nil {
		t.Fatalf("stop after restart: %v", err)
	}
}

func TestFiberHalfTLSFailsFast(t *testing.T) {
	srv := New(
		&server.Options{Name: "fiber-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0", TLSCertPEM: "cert"},
	)
	if err := srv.Start(); !errors.Is(err, ErrIncompleteTLSConfig) {
		_ = srv.Stop()
		t.Fatalf("half-TLS Start must fail fast, got %v", err)
	}
}

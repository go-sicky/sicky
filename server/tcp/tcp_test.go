package tcp

import (
	"testing"

	"github.com/go-sicky/sicky/server"
)

func TestTCPStartStopRestart(t *testing.T) {
	srv := New(
		&server.Options{Name: "tcp-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0", BufferSize: 1024},
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

	// idempotent double stop
	if err := srv.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}

	// restart works after clean stop
	if err := srv.Start(); err != nil {
		t.Fatalf("restart: %v", err)
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("stop after restart: %v", err)
	}
}

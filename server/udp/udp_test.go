package udp

import (
	"testing"

	"github.com/go-sicky/sicky/server"
)

func TestUDPStartStopRestart(t *testing.T) {
	srv := New(
		&server.Options{Name: "udp-test"},
		&Config{Network: "udp", Address: "127.0.0.1:0", BufferSize: 1024},
	)
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if !srv.Running() {
		t.Fatal("not running after Start")
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

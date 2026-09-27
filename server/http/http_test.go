package http

import (
	"errors"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
)

func TestHTTPStartStopRestart(t *testing.T) {
	srv := New(
		&server.Options{Name: "http-test"},
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

func TestHTTPHalfTLSFailsFast(t *testing.T) {
	srv := New(
		&server.Options{Name: "http-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0", TLSCertPEM: "cert"},
	)
	if err := srv.Start(); !errors.Is(err, ErrIncompleteTLSConfig) {
		_ = srv.Stop()
		t.Fatalf("half-TLS Start must fail fast, got %v", err)
	}
}

// TestHTTPInvalidPEMUnlocks guards the lock released on the certificate
// parse failure: Start used to return while holding the write lock, so
// the Stop that follows a failed start deadlocked until SIGKILL.
func TestHTTPInvalidPEMUnlocks(t *testing.T) {
	srv := New(
		&server.Options{Name: "http-test"},
		&Config{
			Network:    "tcp",
			Address:    "127.0.0.1:0",
			TLSCertPEM: "not-a-cert",
			TLSKeyPEM:  "not-a-key",
		},
	)

	if err := srv.Start(); err == nil {
		_ = srv.Stop()
		t.Fatal("unparseable PEM must fail Start")
	}

	// Stop first: every accessor (Running/Addr) takes the read lock, so
	// checking them before Stop would hang on the very bug this guards.
	done := make(chan error, 1)
	go func() {
		done <- srv.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop after failed start: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked: Start left the write lock held")
	}

	if srv.Running() {
		t.Fatal("server must not be running after a failed Start")
	}
}

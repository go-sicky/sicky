package grpc

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/metrics"
)

func TestConfigValidateHalfTLS(t *testing.T) {
	half := &Config{TLSCertPEM: "cert"}
	if err := half.Ensure().Validate(); !errors.Is(err, ErrIncompleteTLSConfig) {
		t.Fatalf("half-TLS must fail validation, got %v", err)
	}

	empty := (&Config{}).Ensure()
	if err := empty.Validate(); err != nil {
		t.Fatalf("empty TLS must validate: %v", err)
	}

	full := &Config{TLSCertPEM: "cert", TLSKeyPEM: "key"}
	if err := full.Ensure().Validate(); err != nil {
		t.Fatalf("full TLS must validate: %v", err)
	}
}

// Half-configured TLS must fail fast (nil client) instead of silently
// falling back to plaintext.
func TestNewHalfTLSFailsFast(t *testing.T) {
	opts := &client.Options{ID: uuid.New(), Name: "tls-test"}
	if got := New(opts, &Config{TLSCertPEM: "cert"}); got != nil {
		_ = got.Disconnect()
		t.Fatal("half-TLS must return nil client")
	}
}

// Bogus PEM material must fail fast as well.
func TestNewBogusPEMmaterialFailsFast(t *testing.T) {
	opts := &client.Options{ID: uuid.New(), Name: "tls-test"}
	if got := New(opts, &Config{TLSCertPEM: "cert", TLSKeyPEM: "key"}); got != nil {
		_ = got.Disconnect()
		t.Fatal("unparseable PEM must return nil client")
	}
}

func TestCallIncrementsCounter(t *testing.T) {
	before := counterValue(metrics.NumGRPCClientCallCounter)
	opts := &client.Options{ID: uuid.New(), Name: "call-test"}
	clt := New(opts, &Config{Addr: "127.0.0.1:1"})
	if clt == nil {
		t.Fatal("direct-addr client must be created")
	}

	defer func() { _ = clt.Disconnect() }()

	if err := clt.Call(); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if got := counterValue(metrics.NumGRPCClientCallCounter); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

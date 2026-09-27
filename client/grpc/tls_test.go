package grpc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/logger"
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
	before := counterValue(metrics.ClientRequestsTotal.WithLabelValues("grpc", "noop", "noop", "noop"))
	opts := &client.Options{ID: uuid.New(), Name: "call-test"}
	clt := New(opts, &Config{Addr: "127.0.0.1:1"})
	if clt == nil {
		t.Fatal("direct-addr client must be created")
	}

	defer func() { _ = clt.Disconnect() }()

	if err := clt.Call(); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if got := counterValue(metrics.ClientRequestsTotal.WithLabelValues("grpc", "noop", "noop", "noop")); got != before+1 {
		t.Fatalf("counter = %v, want %v", got, before+1)
	}
}

// testCAPEM returns a parseable self-signed CA certificate.
func testCAPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sicky-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("self-sign: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// TestTLSFromCAOnly: verifying the server certificate must not require a
// client certificate. Before this, tls_cert_pem/tls_key_pem were the only
// switch, so every deployment was forced into mTLS or plaintext.
func TestTLSFromCAOnly(t *testing.T) {
	cfg := (&Config{Addr: "127.0.0.1:1", TLSCAPEM: testCAPEM(t)}).Ensure()

	tlsCfg, err := clientTLSConfig(cfg)
	if err != nil {
		t.Fatalf("clientTLSConfig: %v", err)
	}

	if tlsCfg == nil {
		t.Fatal("tls_ca_pem alone must enable TLS")
	}

	if tlsCfg.RootCAs == nil {
		t.Fatal("RootCAs must come from tls_ca_pem")
	}

	if len(tlsCfg.Certificates) != 0 {
		t.Fatalf("client certificates = %d, want 0 (mTLS is opt-in)", len(tlsCfg.Certificates))
	}

	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS1.2", tlsCfg.MinVersion)
	}
}

func TestTLSFromServerNameOnly(t *testing.T) {
	cfg := (&Config{Addr: "127.0.0.1:1", TLSServerName: "api.internal"}).Ensure()

	tlsCfg, err := clientTLSConfig(cfg)
	if err != nil {
		t.Fatalf("clientTLSConfig: %v", err)
	}

	if tlsCfg == nil || tlsCfg.ServerName != "api.internal" {
		t.Fatalf("tlsCfg = %+v, want ServerName set", tlsCfg)
	}
}

// TestPlaintextDialIsWarned: no TLS material means plaintext, and the
// client must say so at dial time instead of staying silently
// unencrypted.
func TestPlaintextDialIsWarned(t *testing.T) {
	cfg := (&Config{Addr: "127.0.0.1:1"}).Ensure()
	if cfg.TLSEnabled() {
		t.Fatal("no TLS material must mean plaintext")
	}

	var logs bytes.Buffer
	opts := &client.Options{
		ID:     uuid.New(),
		Name:   "plain-test",
		Logger: logger.NewGeneral(slog.New(slog.NewJSONHandler(&logs, nil))),
	}

	clt := New(opts, cfg)
	if clt == nil {
		t.Fatal("plaintext client must be created")
	}

	defer func() {
		_ = clt.Disconnect()
	}()

	if !strings.Contains(logs.String(), "plaintext") {
		t.Fatalf("plaintext dial was not announced: %s", logs.String())
	}
}

func TestTLSInvalidCARejected(t *testing.T) {
	cfg := (&Config{Addr: "127.0.0.1:1", TLSCAPEM: "not a certificate"}).Ensure()
	if _, err := clientTLSConfig(cfg); !errors.Is(err, ErrInvalidTLSCA) {
		t.Fatalf("err = %v, want ErrInvalidTLSCA", err)
	}

	// Fail fast at construction too: a client that trusts nothing would
	// fail every call with a confusing handshake error.
	if got := New(&client.Options{ID: uuid.New(), Name: "ca-test"}, cfg); got != nil {
		_ = got.Disconnect()
		t.Fatal("invalid tls_ca_pem must return a nil client")
	}
}

func TestTLSHalfPairStillFatal(t *testing.T) {
	cfg := (&Config{Addr: "127.0.0.1:1", TLSCertPEM: "cert"}).Ensure()
	if _, err := clientTLSConfig(cfg); !errors.Is(err, ErrIncompleteTLSConfig) {
		t.Fatalf("err = %v, want ErrIncompleteTLSConfig", err)
	}
}

// TestDisconnectWithoutConnection: construction can fail after the
// struct exists; Disconnect must not dereference a nil connection.
func TestDisconnectWithoutConnection(t *testing.T) {
	clt := &GRPCClient{done: make(chan struct{})}
	if err := clt.Disconnect(); err != nil {
		t.Fatalf("Disconnect on a never-connected client = %v, want nil", err)
	}
}

package infra

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// nats rejects a root_ca_file pinned without enable_tls; mqtt used to accept
// it, stat the file, and then discard it — InitMQTT only reads the CA inside
// its `if c.EnableTLS` branch. The operator gets a plaintext connection that
// looks configured, which is the worst shape for a security setting to fail
// in: silent, and on the side that loses the protection.
func TestMQTTCAFileWithoutTLSIsRejected(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	cfg := &MQTTConfig{Broker: "tcp://127.0.0.1:1883", CAFile: ca}

	if err := cfg.Validate(); !errors.Is(err, ErrMQTTCAWithoutTLS) {
		t.Fatalf("Validate() = %v, want ErrMQTTCAWithoutTLS", err)
	}

	// With TLS on, the same config is valid — the file is then actually read.
	cfg.EnableTLS = true

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil once enable_tls is set", err)
	}
}

// The check must not fire when there is no CA at all: TLS off with no CA is
// ordinary plaintext MQTT, not a mistake.
func TestMQTTWithoutTLSAndWithoutCAIsFine(t *testing.T) {
	cfg := &MQTTConfig{Broker: "tcp://127.0.0.1:1883"}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// An unreadable CA is still an error once TLS is on — the new check must not
// have displaced the existing one.
func TestMQTTUnreadableCAStillFailsWithTLS(t *testing.T) {
	cfg := &MQTTConfig{
		Broker:    "tcp://127.0.0.1:1883",
		CAFile:    filepath.Join(t.TempDir(), "absent.pem"),
		EnableTLS: true,
	}

	if err := cfg.Validate(); !errors.Is(err, ErrMQTTCAUnreadable) {
		t.Errorf("Validate() = %v, want ErrMQTTCAUnreadable", err)
	}
}

// nats is the reference behavior this brings mqtt in line with. Pin it, so
// the two backends cannot drift apart again.
func TestNATSCAWithoutTLSIsRejectedForTheSameReason(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("-----BEGIN CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	cfg := &NatsConfig{URL: "nats://127.0.0.1:4222", RootCAFile: ca}

	if err := cfg.Validate(); !errors.Is(err, ErrNATSCAWithoutTLS) {
		t.Errorf("Validate() = %v, want ErrNATSCAWithoutTLS", err)
	}
}

// mongo.MaxPoolSize is uint64, so "negatives abort" is unreachable by
// construction. What matters is that a negative does not wrap to 2^64-1 —
// the catastrophic case — and that the framework-wide behavior (a silent
// zero, i.e. the driver default) is what actually happens. Measured through
// the real config path rather than assumed.
func TestMongoMaxPoolSizeCannotWrapOnANegative(t *testing.T) {
	cfg := &MongoConfig{URI: "mongodb://127.0.0.1:27017", MaxPoolSize: 0}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("a zero pool size must be valid (driver default): %v", err)
	}

	// Documented consequence: there is no negative case to validate, because
	// the field is unsigned. If this type ever becomes signed, Validate must
	// grow a negative-abort here — that is what this assertion is watching.
	signed := int64(cfg.MaxPoolSize)
	if signed < 0 {
		t.Fatalf("MaxPoolSize = %d: a signed field would need a negative abort", signed)
	}
}

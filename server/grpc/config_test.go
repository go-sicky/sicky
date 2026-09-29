package grpc

import (
	"testing"
	"time"
)

// A bare number in a duration field decodes as nanoseconds. grpc feeds
// these straight into KeepaliveParams, so `connection_timeout: 10`
// tore down every connection after 10ns and failed every RPC. The
// http, fiber and client/grpc configs already normalized; grpc was
// the one miss.
func TestConfigEnsureNormalizesBareNumberDurations(t *testing.T) {
	c := (&Config{
		ConnectionTimeout: 10,
		KeepaliveTime:     30,
		KeepaliveTimeout:  20,
		MaxConnectionIdle: 300,
		MinPingInterval:   5,
	}).Ensure()

	tests := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"connection_timeout", c.ConnectionTimeout, 10 * time.Second},
		{"keepalive_time", c.KeepaliveTime, 30 * time.Second},
		{"keepalive_timeout", c.KeepaliveTimeout, 20 * time.Second},
		{"max_connection_idle", c.MaxConnectionIdle, 300 * time.Second},
		{"min_ping_interval", c.MinPingInterval, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

// A real duration (at least a millisecond) is passed through untouched —
// it is not a bare second count.
func TestConfigEnsureKeepsRealDurations(t *testing.T) {
	c := (&Config{
		ConnectionTimeout: 2 * time.Second,
		KeepaliveTime:     45 * time.Second,
	}).Ensure()

	if c.ConnectionTimeout != 2*time.Second {
		t.Fatalf("real duration mangled: %v", c.ConnectionTimeout)
	}

	if c.KeepaliveTime != 45*time.Second {
		t.Fatalf("real duration mangled: %v", c.KeepaliveTime)
	}
}

func TestConfigEnsureNormalizesShutdownTimeout(t *testing.T) {
	c := (&Config{ShutdownTimeout: 10}).Ensure()

	if c.ShutdownTimeout != 10*time.Second {
		t.Fatalf("shutdown_timeout = %v, want 10s", c.ShutdownTimeout)
	}
}

func TestConfigEnsureShutdownTimeoutDefaults(t *testing.T) {
	// Zero and negative both fall back to the default rather than
	// disabling the bound.
	for _, in := range []time.Duration{0, -1 * time.Second} {
		c := (&Config{ShutdownTimeout: in}).Ensure()

		if c.ShutdownTimeout != DefaultShutdownTimeout {
			t.Fatalf("shutdown_timeout(%v) = %v, want %v", in, c.ShutdownTimeout, DefaultShutdownTimeout)
		}
	}
}

// Negatives on the keepalive knobs are clamped to zero, which gRPC
// reads as "keep the default" — they must not survive as negatives.
func TestConfigEnsureClampsNegativeKeepalive(t *testing.T) {
	c := (&Config{
		ConnectionTimeout:     -1 * time.Second,
		KeepaliveTime:         -1 * time.Second,
		KeepaliveTimeout:      -1 * time.Second,
		MaxConnectionIdle:     -1 * time.Second,
		MinPingInterval:       -1 * time.Second,
		MaxConnectionAgeGrace: -1 * time.Second,
	}).Ensure()

	tests := []struct {
		name string
		got  time.Duration
	}{
		{"connection_timeout", c.ConnectionTimeout},
		{"keepalive_time", c.KeepaliveTime},
		{"keepalive_timeout", c.KeepaliveTimeout},
		{"max_connection_idle", c.MaxConnectionIdle},
		{"min_ping_interval", c.MinPingInterval},
		{"max_connection_age_grace", c.MaxConnectionAgeGrace},
	}

	for _, tt := range tests {
		if tt.got < 0 {
			t.Fatalf("%s = %v, want >= 0", tt.name, tt.got)
		}
	}
}

func TestConfigEnsureNilSafe(t *testing.T) {
	c := (*Config)(nil).Ensure()

	if c == nil {
		t.Fatal("nil Ensure returned nil")
	}

	if c.Network != DefaultNetwork {
		t.Fatalf("network = %q, want %q", c.Network, DefaultNetwork)
	}
}

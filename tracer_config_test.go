package sicky

import (
	"errors"
	"testing"
)

func TestTracerConfigEnsure(t *testing.T) {
	var c *TracerConfig
	c = c.Ensure()
	if c.Type != DefaultTracerType {
		t.Fatalf("nil Ensure type = %q", c.Type)
	}

	c = &TracerConfig{SampleRate: 9}
	c.Ensure()
	if c.SampleRate != 1.0 {
		t.Fatalf("out-of-range sample rate not clamped: %v", c.SampleRate)
	}
}

func TestTracerConfigValidate(t *testing.T) {
	if err := (&TracerConfig{Type: "none"}).Validate(); err != nil {
		t.Fatalf("none must validate: %v", err)
	}

	if err := (&TracerConfig{Type: "grpc"}).Validate(); err != nil {
		t.Fatalf("grpc must validate: %v", err)
	}

	if err := (&TracerConfig{Type: "bogus"}).Validate(); !errors.Is(err, ErrTracerUnknownType) {
		t.Fatalf("bogus type must fail with ErrTracerUnknownType, got %v", err)
	}

	if err := (&TracerConfig{Type: "uptrace"}).Validate(); !errors.Is(err, ErrTracerNoDSN) {
		t.Fatalf("uptrace without DSN must fail with ErrTracerNoDSN, got %v", err)
	}

	if err := (&TracerConfig{Type: "uptrace", DSN: "https://t@host/1"}).Validate(); err != nil {
		t.Fatalf("uptrace with DSN must validate: %v", err)
	}
}

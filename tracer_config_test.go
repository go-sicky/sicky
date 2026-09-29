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

// An absent sample_rate decodes to 0.0 and 0.0 reached
// TraceIDRatioBased(0), so the tracer exported no spans at all while
// Run() still logged "tracer initialized".
func TestTracerConfigEnsureZeroSampleRateFillsDefault(t *testing.T) {
	c := (&TracerConfig{Type: "grpc", Endpoint: "otel:4317"}).Ensure()

	if c.SampleRate != DefaultTracerSampleRate {
		t.Fatalf("zero sample rate = %v, want %v", c.SampleRate, DefaultTracerSampleRate)
	}

	if c.SampleRate <= 0 {
		t.Fatalf("sample rate %v would sample zero spans", c.SampleRate)
	}
}

func TestTracerConfigEnsureKeepsValidSampleRate(t *testing.T) {
	c := (&TracerConfig{SampleRate: 0.25}).Ensure()

	if c.SampleRate != 0.25 {
		t.Fatalf("valid sample rate clobbered: %v", c.SampleRate)
	}
}

func TestTracerConfigValidate(t *testing.T) {
	if err := (&TracerConfig{Type: "none"}).Validate(); err != nil {
		t.Fatalf("none must validate: %v", err)
	}

	if err := (&TracerConfig{Type: "grpc"}).Validate(); !errors.Is(err, ErrTracerNoEndpoint) {
		t.Fatalf("grpc without endpoint must fail with ErrTracerNoEndpoint, got %v", err)
	}

	if err := (&TracerConfig{Type: "http"}).Validate(); !errors.Is(err, ErrTracerNoEndpoint) {
		t.Fatalf("http without endpoint must fail with ErrTracerNoEndpoint, got %v", err)
	}

	if err := (&TracerConfig{Type: "grpc", Endpoint: "otel:4317"}).Validate(); err != nil {
		t.Fatalf("grpc with endpoint must validate: %v", err)
	}

	if err := (&TracerConfig{Type: "http", Endpoint: "otel:4318"}).Validate(); err != nil {
		t.Fatalf("http with endpoint must validate: %v", err)
	}

	// stdout is local and needs no endpoint.
	if err := (&TracerConfig{Type: "stdout"}).Validate(); err != nil {
		t.Fatalf("stdout must validate without an endpoint: %v", err)
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

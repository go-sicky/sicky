package internal

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

func TestClampSampleRate(t *testing.T) {
	for in, want := range map[float64]float64{
		0.0: 0.0, 0.5: 0.5, 1.0: 1.0,
		-0.1: 1.0, 1.1: 1.0, 9.0: 1.0,
	} {
		if got := ClampSampleRate(in); got != want {
			t.Fatalf("Clamp(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestNewOTLPProvider(t *testing.T) {
	exp, err := stdouttrace.New(stdouttrace.WithoutTimestamps())
	if err != nil {
		t.Fatalf("stdout exporter: %v", err)
	}

	p, err := NewOTLPProvider("svc", "v1", "instance-1", 1.0, exp)
	if err != nil || p == nil {
		t.Fatalf("provider = %v, err = %v", p, err)
	}

	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

package internal

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
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

	p, err := NewOTLPProvider("svc", "v1", "instance-1", 1.0, false, exp)
	if err != nil || p == nil {
		t.Fatalf("provider = %v, err = %v", p, err)
	}

	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

// samplingParams builds server-side parameters with the given parent
// context.
func samplingParams(ctx context.Context) sdktrace.SamplingParameters {
	return sdktrace.SamplingParameters{
		ParentContext: ctx,
		TraceID:       oteltrace.TraceID{1, 2, 3},
		Name:          "GET /x",
		Kind:          oteltrace.SpanKindServer,
	}
}

func remoteParent(sampled bool) context.Context {
	flags := oteltrace.TraceFlags(0)
	if sampled {
		flags = oteltrace.FlagsSampled
	}

	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{1, 2, 3},
		SpanID:     oteltrace.SpanID{4, 5, 6},
		TraceFlags: flags,
		Remote:     true,
	})

	return oteltrace.ContextWithSpanContext(context.Background(), sc)
}

func localParent() context.Context {
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{1, 2, 3},
		SpanID:     oteltrace.SpanID{4, 5, 6},
		TraceFlags: oteltrace.FlagsSampled,
	})

	return oteltrace.ContextWithSpanContext(context.Background(), sc)
}

// TestSamplerIgnoresRemoteSampledFlag: with sample_rate 0 an attacker
// sending `traceparent: ...-01` used to be sampled anyway (ParentBased
// honors the header), turning the sampling configuration into a cost
// DoS. The remote flag is now replaced by this service's own ratio.
func TestSamplerIgnoresRemoteSampledFlag(t *testing.T) {
	if got := NewSampler(0, false).ShouldSample(samplingParams(remoteParent(true))).Decision; got != sdktrace.Drop {
		t.Fatalf("untrusted sampled parent with rate 0 -> %v, want Drop", got)
	}

	// The same request with trust_remote_sampled keeps the legacy
	// header-driven behavior for trusted meshes.
	if got := NewSampler(0, true).ShouldSample(samplingParams(remoteParent(true))).Decision; got == sdktrace.Drop {
		t.Fatal("trusted sampled parent must be sampled")
	}
}

// TestSamplerFollowsLocalParent: children of our own spans must still be
// sampled with the parent, regardless of the ratio.
func TestSamplerFollowsLocalParent(t *testing.T) {
	if got := NewSampler(0, false).ShouldSample(samplingParams(localParent())).Decision; got == sdktrace.Drop {
		t.Fatal("local sampled parent must be sampled")
	}
}

// TestSamplerRespectsUpstreamDrop: an unsampled remote parent stays
// unsampled so exported spans never appear without their parents.
func TestSamplerRespectsUpstreamDrop(t *testing.T) {
	if got := NewSampler(1, false).ShouldSample(samplingParams(remoteParent(false))).Decision; got != sdktrace.Drop {
		t.Fatalf("unsampled remote parent with rate 1 -> %v, want Drop", got)
	}
}

// TestSamplerHonoursOwnRatio: with trust off, an upstream sampled flag
// still goes through the configured ratio at rate 1.
func TestSamplerHonoursOwnRatio(t *testing.T) {
	if got := NewSampler(1, false).ShouldSample(samplingParams(remoteParent(true))).Decision; got == sdktrace.Drop {
		t.Fatal("rate 1 must sample a sampled remote parent")
	}
}

package tracer

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"abc-123_.:XYZ": "abc-123_.:XYZ",
		"  padded  ":    "padded",
		"has space":     "",
		"a/b":           "",
		"CR\rLF\n":      "",
		"":              "",
	}

	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Fatalf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}

	long := strings.Repeat("a", 200)
	if got := Sanitize(long); len(got) != maxSanitizedValueLen {
		t.Fatalf("long value not truncated: len=%d", len(got))
	}
}

func TestRedactDSN(t *testing.T) {
	if got := RedactDSN("https://token123@uptrace.example.com/1"); got != "https://uptrace.example.com/1" {
		t.Fatalf("redact = %q", got)
	}

	if got := RedactDSN("127.0.0.1:4317"); got != "127.0.0.1:4317" {
		t.Fatalf("plain endpoint must pass through, got %q", got)
	}

	if got := RedactDSN(""); got != "" {
		t.Fatalf("empty must pass through, got %q", got)
	}
}

// RedactDSN failed open: a DSN with no "@" was returned verbatim, so a
// credential carried in the query string reached the log stream. Managed OTLP
// vendors accept the key exactly there (…/v1/traces?api-key=…), and
// tracer/uptrace logs the DSN at seven sites.
func TestRedactDSNFailsClosed(t *testing.T) {
	cases := []struct {
		in     string
		secret string
	}{
		{"uptrace://host/1?token=abc", "abc"},
		{"https://otlp.example.com/v1/traces?api-key=SECRET", "SECRET"},
		{"https://otlp.example.com/v1/traces?x-api-key=SECRET", "SECRET"},
		{"collector:4317?access_token=TOK", "TOK"},
		{"https://otlp.example.com/v1/traces?signature=SIG", "SIG"},
	}

	for _, c := range cases {
		got := RedactDSN(c.in)
		if strings.Contains(got, c.secret) {
			t.Errorf("RedactDSN(%q) = %q, leaks %q", c.in, got, c.secret)
		}
	}

	// The collector address must survive: redacting it away makes the log
	// useless for the common case.
	if got := RedactDSN("https://otlp.example.com:4317"); got != "https://otlp.example.com:4317" {
		t.Errorf("credential-free endpoint = %q, want it preserved", got)
	}
}

func TestPropagatorRoundTrip(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	defer func() {
		_ = provider.Shutdown(context.Background())
	}()
	tr := provider.Tracer("test")

	ctx, span := tr.Start(context.Background(), "op")
	carrier := propagation.MapCarrier{}
	Inject(ctx, carrier)
	span.End()

	if carrier.Get("traceparent") == "" {
		t.Fatalf("Inject must emit W3C traceparent, carrier=%v", map[string]string(carrier))
	}

	if carrier.Get("x-b3-traceid") == "" && carrier.Get("X-B3-TraceId") == "" {
		t.Fatalf("Inject must emit B3 headers, carrier=%v", map[string]string(carrier))
	}

	extracted := Extract(context.Background(), carrier)
	got := propagation.MapCarrier{}
	Inject(extracted, got)
	if got.Get("traceparent") != carrier.Get("traceparent") {
		t.Fatalf("traceparent not preserved: %q vs %q", got.Get("traceparent"), carrier.Get("traceparent"))
	}
}

// BenchmarkPropagatorIsBuiltOnce measures what the per-call rebuild used to
// cost. Propagator is called once per inbound request (Extract) and once per
// outbound span (Inject), from five server and client call sites, so the
// composite was being reconstructed on every one of them.
//
// Run with -benchmem to see the allocations the sync.OnceValue removed.
func BenchmarkPropagatorIsBuiltOnce(b *testing.B) {
	b.ReportAllocs()

	for range b.N {
		_ = Propagator()
	}
}

// Propagator must not rebuild the composite per call. The identity check that
// would read most directly cannot be written — compositeTextMapPropagator
// holds a slice and is therefore uncomparable — so the property is asserted
// the way it is actually paid for: in allocations.
//
// Rebuilding cost 4 allocs / 89 B per call (b3.New's config plus the slice
// header), on a path reached once per inbound request and once per outbound
// span. sync.OnceValue drops that to zero, so anything above a couple of
// allocs means the cache is gone.
func TestPropagatorIsNotRebuiltPerCall(t *testing.T) {
	// Warm the OnceValue so its own first-call cost is not measured.
	_ = Propagator()

	allocs := testing.AllocsPerRun(200, func() { _ = Propagator() })
	if allocs > 0 {
		t.Errorf("Propagator() allocates %v times per call, want 0: the composite "+
			"must be built once, not per inbound request or outbound span", allocs)
	}
}

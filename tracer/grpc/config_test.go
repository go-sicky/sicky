/**
 * @file config_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 09/30/2026
 */

package grpc

import "testing"

// TestConfigEnsureFillsDefaults pins the default fill. The zero Config is
// what a caller gets from `&Config{}`, so every default it leaves unset is
// a value the exporter will run with.
func TestConfigEnsureFillsDefaults(t *testing.T) {
	c := (&Config{}).Ensure()

	if c.Endpoint != DefaultEndpoint {
		t.Fatalf("Endpoint = %q, want %q", c.Endpoint, DefaultEndpoint)
	}

	if c.ServiceName != DefaultServiceName {
		t.Fatalf("ServiceName = %q, want %q", c.ServiceName, DefaultServiceName)
	}

	if c.ServiceVersion != DefaultServiceVersion {
		t.Fatalf("ServiceVersion = %q, want %q", c.ServiceVersion, DefaultServiceVersion)
	}

	// SampleRate is filled too: Ensure refills a non-positive rate, so a
	// zero Config no longer reaches TraceIDRatioBased(0) and silently
	// exports nothing. Pinned by TestConfigEnsureRefillsZeroSampleRate.
	if c.SampleRate != DefaultSampleRate {
		t.Fatalf("SampleRate = %v, want %v", c.SampleRate, DefaultSampleRate)
	}
}

// TestConfigEnsureIsNilSafe: a nil receiver must not panic, because Run
// hands the config straight through without a nil check of its own.
func TestConfigEnsureIsNilSafe(t *testing.T) {
	var nilCfg *Config

	got := nilCfg.Ensure()
	if got == nil {
		t.Fatal("Ensure on a nil receiver must return a usable config")
	}

	if got.Endpoint != DefaultEndpoint {
		t.Fatalf("Endpoint = %q, want %q", got.Endpoint, DefaultEndpoint)
	}
}

// TestConfigEnsureClampsOutOfRangeSampleRate: a rate outside [0,1] is a
// typo or a copy-paste from a percentage, and a negative one would reach
// TraceIDRatioBased as a nonsense ratio.
func TestConfigEnsureClampsOutOfRangeSampleRate(t *testing.T) {
	tests := []struct {
		name string
		in   float64
	}{
		{name: "above one", in: 1.5},
		{name: "negative", in: -0.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := (&Config{SampleRate: tc.in}).Ensure()
			if got.SampleRate != DefaultSampleRate {
				t.Fatalf("SampleRate(%v) = %v, want %v", tc.in, got.SampleRate, float64(DefaultSampleRate))
			}
		})
	}
}

// TestConfigEnsureKeepsExplicitSampleRate: a deliberate partial rate must
// survive, or head-based sampling becomes impossible to configure.
func TestConfigEnsureKeepsExplicitSampleRate(t *testing.T) {
	for _, rate := range []float64{0.01, 0.5, 1.0} {
		got := (&Config{SampleRate: rate}).Ensure()
		if got.SampleRate != rate {
			t.Fatalf("SampleRate = %v, want %v: Ensure must not rewrite an in-range rate", got.SampleRate, rate)
		}
	}
}

// TestConfigEnsureRefillsZeroSampleRate is a regression test for the silent no-op tracer. A bare float64
// config key the operator omitted decodes to 0, and 0 used to reach
// sdktrace.TraceIDRatioBased(0), which never samples: the tracer reported
// itself initialized and exported nothing at all. Ensure now refills a
// non-positive rate with the default, the same rule the root
// sicky.TracerConfig.Ensure applies, so a process run through sicky.Run
// and one that builds this tracer directly agree.
//
// The cost is that a deliberate 0% is not expressible at this level; a
// caller that wants less traffic lowers the rate above zero instead.
func TestConfigEnsureRefillsZeroSampleRate(t *testing.T) {
	if got := (&Config{SampleRate: 0}).Ensure().SampleRate; got != DefaultSampleRate {
		t.Fatalf("SampleRate = %v, want %v: a zero rate must be refilled, not passed to TraceIDRatioBased(0), "+
			"which samples nothing", got, DefaultSampleRate)
	}
}

// TestConfigDoesNotTrustRemoteSamplingByDefault is a security regression
// test. TrustRemoteSampled lets a client-chosen traceparent decide
// sampling here; if it ever defaulted to true, any remote peer could set
// the sampled flag and sidestep sample_rate, and with it the export
// budget that keeps a misbehaving or hostile client from flooding the
// collector.
func TestConfigDoesNotTrustRemoteSamplingByDefault(t *testing.T) {
	if (&Config{}).Ensure().TrustRemoteSampled {
		t.Fatal("TrustRemoteSampled defaults to true: a client-chosen traceparent could bypass sample_rate")
	}

	if DefaultConfig().TrustRemoteSampled {
		t.Fatal("DefaultConfig enables TrustRemoteSampled: the same sampling bypass applies to the defaults")
	}

	// Ensure must not flip an explicit opt-out either.
	if (&Config{TrustRemoteSampled: false}).Ensure().TrustRemoteSampled {
		t.Fatal("Ensure turned TrustRemoteSampled on for a config that left it false")
	}
}

// TestConfigPreservesInsecureAndHeaders: Insecure selects a plaintext
// exporter connection and Headers carry the collector credentials, so
// neither may be silently reset by Ensure.
func TestConfigPreservesInsecureAndHeaders(t *testing.T) {
	hdrs := map[string]string{"authorization": "Bearer secret"}

	got := (&Config{Insecure: false, Headers: hdrs, Timeout: 5}).Ensure()

	if got.Insecure {
		t.Fatal("Ensure flipped Insecure back to true: an operator asking for TLS lost it")
	}

	if got.Headers["authorization"] != "Bearer secret" {
		t.Fatalf("Headers = %v, want the configured credentials preserved", got.Headers)
	}

	if got.Timeout != 5 {
		t.Fatalf("Timeout = %d, want 5", got.Timeout)
	}
}

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

	// SampleRate is deliberately absent here. Ensure only clamps values
	// OUTSIDE [0,1]; a zero in-range value is left alone, so
	// DefaultSampleRate comes from DefaultConfig (reached by a nil
	// receiver), not from Ensure. That asymmetry is pinned by
	// TestConfigEnsureKeepsZeroSampleRateAsDocumented.
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

// TestConfigEnsureKeepsZeroSampleRateAsDocumented pins the current
// behavior, which differs from the root sicky.TracerConfig: here the guard
// is `> 1.0 || < 0.0`, so an explicit 0.0 survives Ensure.
//
// This matters because 0.0 reaches sdktrace.TraceIDRatioBased(0) and
// exports nothing. sicky.Run is safe because the root TracerConfig.Ensure
// now refills a zero rate with DefaultTracerSampleRate before it reaches
// this package, but a caller that constructs a tracer directly with
// `grpc.New(nil, &Config{})` gets the silent no-op. If this test ever
// needs to change, the fix belongs here too, not only at the root.
func TestConfigEnsureKeepsZeroSampleRateAsDocumented(t *testing.T) {
	got := (&Config{SampleRate: 0}).Ensure()
	if got.SampleRate != 0 {
		t.Fatalf("SampleRate = %v, want 0: the sub-package guard is `> 1.0 || < 0.0` and does not refill zero; "+
			"sicky.Run is protected by the root config, a direct New(nil, &Config{}) caller is not", got.SampleRate)
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

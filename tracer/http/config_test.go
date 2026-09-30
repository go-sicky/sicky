/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file config_test.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/30/2026
 */

package http

import "testing"

func TestConfigEnsureFillsDefaults(t *testing.T) {
	c := (&Config{}).Ensure()

	if c.Endpoint != DefaultEndpoint {
		t.Fatalf("Endpoint = %q, want %q", c.Endpoint, DefaultEndpoint)
	}

	if c.ServiceName != DefaultServiceName || c.ServiceVersion != DefaultServiceVersion {
		t.Fatalf("names = %q/%q, want %q/%q", c.ServiceName, c.ServiceVersion, DefaultServiceName, DefaultServiceVersion)
	}

	// SampleRate is deliberately absent here. Ensure only clamps values
	// OUTSIDE [0,1]; a zero in-range value is left alone, so
	// DefaultSampleRate comes from DefaultConfig (reached by a nil
	// receiver), not from Ensure. That asymmetry is pinned by
	// TestConfigEnsureRefillsZeroSampleRate.
}

func TestConfigEnsureIsNilSafe(t *testing.T) {
	var nilCfg *Config

	c := nilCfg.Ensure()
	if c == nil {
		t.Fatal("nil Ensure must return a non-nil config")
	}

	if c.Endpoint != DefaultEndpoint || c.SampleRate != DefaultSampleRate {
		t.Fatalf("nil Ensure must fall back to DefaultConfig, got %+v", c)
	}
}

func TestConfigEnsureClampsOutOfRangeSampleRate(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "above one", in: 1.5, want: DefaultSampleRate},
		{name: "negative", in: -0.5, want: DefaultSampleRate},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (&Config{SampleRate: tc.in}).Ensure().SampleRate; got != tc.want {
				t.Fatalf("SampleRate(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestConfigEnsureKeepsExplicitSampleRate(t *testing.T) {
	for _, want := range []float64{0.01, 0.5, 1.0} {
		if got := (&Config{SampleRate: want}).Ensure().SampleRate; got != want {
			t.Fatalf("SampleRate = %v, want the explicit %v", got, want)
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

// TestConfigDoesNotTrustRemoteSamplingByDefault is the security regression:
// accepting a client-chosen sampled flag would let a caller bypass
// sample_rate entirely, and with it the export budget the operator sized the
// collector for.
func TestConfigDoesNotTrustRemoteSamplingByDefault(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "zero value", cfg: &Config{}},
		{name: "default config", cfg: DefaultConfig()},
		{name: "explicit false", cfg: &Config{TrustRemoteSampled: false}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cfg.Ensure().TrustRemoteSampled {
				t.Fatal("TrustRemoteSampled must default to false so a client traceparent cannot bypass sample_rate")
			}
		})
	}
}

func TestConfigPreservesInsecureAndHeaders(t *testing.T) {
	headers := map[string]string{"x-api-key": "secret"}

	c := (&Config{Insecure: false, Headers: headers, Timeout: 7}).Ensure()

	if c.Insecure {
		t.Fatal("Ensure must not turn Insecure back on; an explicit false is a security choice")
	}

	if c.Headers["x-api-key"] != "secret" || len(c.Headers) != 1 {
		t.Fatalf("Headers = %v, want the operator's map untouched", c.Headers)
	}

	if c.Timeout != 7 {
		t.Fatalf("Timeout = %d, want the explicit 7", c.Timeout)
	}
}

func TestDefaultConfigUsesLoopbackEndpoint(t *testing.T) {
	// The exporter ships pointing at a loopback collector, never at a
	// routable host, so a misconfigured deployment fails closed.
	if DefaultConfig().Endpoint != "127.0.0.1:4318" {
		t.Fatalf("default endpoint = %q, want the loopback OTLP HTTP collector", DefaultConfig().Endpoint)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

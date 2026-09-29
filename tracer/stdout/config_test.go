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
 * @package stdout
 * @author Dr.NP <np@herewe.tech>
 * @since 09/30/2026
 */

package stdout

import "testing"

func TestConfigEnsureFillsDefaults(t *testing.T) {
	c := (&Config{}).Ensure()

	if c.ServiceName != DefaultServiceName || c.ServiceVersion != DefaultServiceVersion {
		t.Fatalf("names = %q/%q, want %q/%q", c.ServiceName, c.ServiceVersion, DefaultServiceName, DefaultServiceVersion)
	}

	// SampleRate is deliberately absent here. Ensure only clamps values
	// OUTSIDE [0,1]; a zero in-range value is left alone, so
	// DefaultSampleRate comes from DefaultConfig (reached by a nil
	// receiver), not from Ensure. That asymmetry is pinned by
	// TestConfigEnsureKeepsZeroSampleRateAsDocumented.

	// Timestamps is also absent: Ensure does not fill it, so a zero
	// Config keeps timestamps off even though DefaultConfig turns them on.
	// That split is pinned by TestConfigEnsureLeavesTimestampsAlone.
}

func TestConfigEnsureIsNilSafe(t *testing.T) {
	var nilCfg *Config

	c := nilCfg.Ensure()
	if c == nil {
		t.Fatal("nil Ensure must return a non-nil config")
	}

	if c.ServiceName != DefaultServiceName || c.SampleRate != DefaultSampleRate {
		t.Fatalf("nil Ensure must fall back to DefaultConfig, got %+v", c)
	}
}

// TestConfigEnsureLeavesTimestampsAlone pins the DefaultConfig/Ensure split:
// DefaultConfig enables timestamps but Ensure does not fill them, so a
// zero Config and a nil Config differ in output format. The behavior is
// asymmetric but intentional: a caller that asks for no timestamps gets
// none, and only the default path is opinionated.
func TestConfigEnsureLeavesTimestampsAlone(t *testing.T) {
	if (&Config{}).Ensure().Timestamps {
		t.Fatal("Ensure must not turn Timestamps on for an explicit config; only DefaultConfig does")
	}

	if !DefaultConfig().Timestamps {
		t.Fatal("DefaultConfig must enable Timestamps")
	}
}

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
			if got := (&Config{SampleRate: tc.in}).Ensure().SampleRate; got != DefaultSampleRate {
				t.Fatalf("SampleRate(%v) = %v, want %v", tc.in, got, DefaultSampleRate)
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

func TestConfigEnsureKeepsZeroSampleRateAsDocumented(t *testing.T) {
	// A zero is inside the accepted range, so Ensure leaves it alone. See
	// the same note in tracer/http: sicky.TracerConfig.Ensure treats zero
	// as "unset" because viper decodes an absent float64 to zero, this
	// package does not.
	if got := (&Config{SampleRate: 0}).Ensure().SampleRate; got != 0 {
		t.Fatalf("SampleRate = %v, want 0 preserved (in-range zero is not clamped)", got)
	}
}

// TestConfigDoesNotTrustRemoteSamplingByDefault is the security regression:
// accepting a client-chosen sampled flag would let a caller bypass
// sample_rate entirely, and with it the export budget.
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

func TestConfigPreservesPrettyPrint(t *testing.T) {
	if (&Config{PrettyPrint: true}).Ensure().PrettyPrint != true {
		t.Fatal("PrettyPrint must be preserved when explicitly enabled")
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

package utils

import (
	"testing"
	"time"
)

// TestNormalizeDurationRepairsBareSecondCounts pins the repair the helper
// exists for: a bare int in a time.Duration field decodes as nanoseconds.
// Note the arithmetic is int64(d) * int64(time.Second), so it multiplies
// the raw nanosecond value: 10 means 10s. A real (if unusual) sub-second
// duration is therefore rescaled too, which is why these cases use the
// bare ints a config file actually produces.
func TestNormalizeDurationRepairsBareSecondCounts(t *testing.T) {
	for _, tc := range []struct {
		in   time.Duration
		want time.Duration
	}{
		{10, 10 * time.Second},
		{30, 30 * time.Second},
		{1, time.Second},
	} {
		if got := NormalizeDuration(tc.in); got != tc.want {
			t.Fatalf("NormalizeDuration(%v) = %v, want %v: a bare int must read as seconds", tc.in, got, tc.want)
		}
	}
}

// TestNormalizeDurationLeavesCallerOwnedValuesAlone pins the half of the
// contract that is not a repair: zero and negative values are returned
// untouched so each caller can apply its own policy. There is no
// Validate that rejects a negative anywhere in the repo, so a future
// change that clamps here would silently override those policies.
func TestNormalizeDurationLeavesCallerOwnedValuesAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   time.Duration
	}{
		{"zero means use the default", 0},
		{"negative is the caller's to handle", -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeDuration(tc.in); got != tc.in {
				t.Fatalf("NormalizeDuration(%v) = %v, want it returned untouched", tc.in, got)
			}
		})
	}
}

// TestNormalizeDurationKeepsRealDurations guards against over-eager
// repair: a duration written as a proper time string is already in the
// right unit and must survive untouched.
func TestNormalizeDurationKeepsRealDurations(t *testing.T) {
	for _, in := range []time.Duration{time.Millisecond, 2 * time.Second, 45 * time.Second, time.Hour} {
		if got := NormalizeDuration(in); got != in {
			t.Fatalf("NormalizeDuration(%v) = %v, want unchanged", in, got)
		}
	}
}

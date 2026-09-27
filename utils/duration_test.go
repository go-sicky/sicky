package utils

import (
	"testing"
	"time"
)

func TestNormalizeDuration(t *testing.T) {
	for _, tt := range []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, 0},                               // zero means "use the default"
		{-1, -1},                             // negative stays for Validate to abort on
		{10, 10 * time.Second},               // bare seconds from JSON/viper
		{1, time.Second},                     // smallest bare value
		{999, 999 * time.Second},             // still below the threshold
		{time.Millisecond, time.Millisecond}, // threshold itself
		{10 * time.Second, 10 * time.Second}, // explicit duration, untouched
		{1500 * time.Millisecond, 1500 * time.Millisecond},
	} {
		if got := NormalizeDuration(tt.in); got != tt.want {
			t.Errorf("NormalizeDuration(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

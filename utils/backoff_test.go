package utils

import (
	"testing"
	"time"
)

func TestBackoffGrowthAndCap(t *testing.T) {
	b := NewBackoff(50*time.Millisecond, time.Second)
	var prev time.Duration
	for i := range 10 {
		d := b.Next()
		if d <= 0 || d > 1250*time.Millisecond {
			t.Fatalf("step %d: backoff %v out of [>0, cap+jitter]", i, d)
		}

		if i > 1 && d < prev/2 {
			t.Fatalf("step %d: backoff shrank unexpectedly %v -> %v", i, prev, d)
		}

		prev = d
	}

	b.Reset()
	if d := b.Next(); d > 100*time.Millisecond {
		t.Fatalf("after Reset first backoff = %v, want ~50ms+jitter", d)
	}
}

func TestBackoffDefaults(t *testing.T) {
	b := NewBackoff(0, 0)
	if d := b.Next(); d <= 0 || d > 1250*time.Millisecond {
		t.Fatalf("default backoff = %v", d)
	}
}

func TestLogSamplerGates(t *testing.T) {
	s := NewLogSampler(2, 50*time.Millisecond)
	if ok, _ := s.Allow(); !ok {
		t.Fatal("first allow")
	}

	if ok, _ := s.Allow(); !ok {
		t.Fatal("second allow")
	}

	if ok, _ := s.Allow(); ok {
		t.Fatal("third must suppress")
	}

	if ok, supp := s.Allow(); ok || supp != 0 {
		t.Fatalf("suppressed Allow must return (false, 0), got (%v, %d)", ok, supp)
	}

	time.Sleep(60 * time.Millisecond)
	if ok, supp := s.Allow(); !ok || supp != 0 {
		t.Fatalf("fresh window must allow with 0 suppressed, got (%v, %d)", ok, supp)
	}
}

package fiber

import (
	"strings"
	"testing"
	"time"
)

func TestConfigEnsureDefaults(t *testing.T) {
	c := (&Config{}).Ensure()
	if c.Network != DefaultNetwork || c.Address != DefaultAddress {
		t.Fatalf("network/address defaults: %+v", c)
	}
	if c.BodyLimit != DefaultBodyLimit {
		t.Fatalf("body_limit default = %d, want %d", c.BodyLimit, DefaultBodyLimit)
	}
	if c.ReadTimeout != DefaultReadTimeout || c.WriteTimeout != DefaultWriteTimeout || c.IdleTimeout != DefaultIdleTimeout {
		t.Fatalf("timeout defaults: %+v", c)
	}
	if c.CORS == nil || c.CORS.MaxAge != DefaultCORSMaxAge {
		t.Fatalf("cors defaults: %+v", c.CORS)
	}
	if len(c.CORS.AllowedOrigins) != 0 {
		t.Fatalf("cors must deny by default, got %v", c.CORS.AllowedOrigins)
	}
}

func TestConfigEnsureClamps(t *testing.T) {
	c := (&Config{
		BodyLimit:       -1,
		Concurrency:     -2,
		ReadBufferSize:  -3,
		WriteBufferSize: -4,
		ReadTimeout:     -time.Second,
		ShutdownTimeout: -time.Second,
	}).Ensure()
	if c.BodyLimit != DefaultBodyLimit {
		t.Fatalf("body_limit clamp = %d", c.BodyLimit)
	}
	if c.Concurrency != DefaultConcurrency {
		t.Fatalf("concurrency clamp = %d", c.Concurrency)
	}
	if c.ReadBufferSize != DefaultReadBufferSize || c.WriteBufferSize != DefaultWriteBufferSize {
		t.Fatalf("buffer clamp: %+v", c)
	}
	if c.ReadTimeout != DefaultReadTimeout || c.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("timeout clamp: %+v", c)
	}
}

func TestSanitizePropagatedValue(t *testing.T) {
	cases := map[string]string{
		"abc-123_.:XYZ": "abc-123_.:XYZ",
		"  padded  ":    "padded",
		"has space":     "",
		"a/b":           "",
		"CR\rLF\n":      "",
		"":              "",
	}
	for in, want := range cases {
		if got := sanitizePropagatedValue(in); got != want {
			t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("a", 200)
	if got := sanitizePropagatedValue(long); len(got) != maxPropagatedValueLen {
		t.Fatalf("long value not truncated: len=%d", len(got))
	}
}

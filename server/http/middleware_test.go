package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uptrace/bunrouter"
)

func TestConfigEnsureDefaults(t *testing.T) {
	c := (&Config{}).Ensure()
	if c.Network != DefaultNetwork || c.Address != DefaultAddress {
		t.Fatalf("network/address defaults: %+v", c)
	}

	if c.ReadTimeout != DefaultReadTimeout || c.ReadHeaderTimeout != DefaultReadHeaderTimeout ||
		c.WriteTimeout != DefaultWriteTimeout || c.IdleTimeout != DefaultIdleTimeout {
		t.Fatalf("timeout defaults: %+v", c)
	}

	if c.MaxHeaderBytes != DefaultMaxHeaderBytes {
		t.Fatalf("max_header_bytes default = %d", c.MaxHeaderBytes)
	}

	if c.BodyLimit != DefaultBodyLimit {
		t.Fatalf("body_limit default = %d", c.BodyLimit)
	}

	if c.CORS == nil || c.CORS.MaxAge != DefaultCORSMaxAge {
		t.Fatalf("cors defaults: %+v", c.CORS)
	}

	if c.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("shutdown_timeout default = %v", c.ShutdownTimeout)
	}
}

func TestConfigEnsureClamps(t *testing.T) {
	c := (&Config{
		ReadTimeout:    -time.Second,
		MaxHeaderBytes: -1,
		BodyLimit:      -1,
	}).Ensure()
	if c.ReadTimeout != DefaultReadTimeout {
		t.Fatalf("read_timeout clamp = %v", c.ReadTimeout)
	}

	if c.MaxHeaderBytes != DefaultMaxHeaderBytes {
		t.Fatalf("max_header_bytes clamp = %d", c.MaxHeaderBytes)
	}

	if c.BodyLimit != DefaultBodyLimit {
		t.Fatalf("body_limit clamp = %d", c.BodyLimit)
	}
}

func TestSanitizePropagatedValue(t *testing.T) {
	cases := map[string]string{
		"abc-123_.:XYZ": "abc-123_.:XYZ",
		"  padded  ":    "padded",
		"has space":     "",
		"a/b":           "",
		"":              "",
	}

	for in, want := range cases {
		if got := sanitizePropagatedValue(in); got != want {
			t.Fatalf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func callCORS(t *testing.T, cfg *CORSConfig, method, origin string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "http://x.test/", http.NoBody)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	h := NewCORSMiddleware(cfg)(func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusTeapot)

		return nil
	})
	if err := h(w, bunrouter.NewRequest(req)); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	return w
}

func TestCORSMiddlewareWhitelist(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"https://app.test"}, AllowCredentials: true}

	// Allowed origin: echoed + credentials + vary.
	w := callCORS(t, cfg, http.MethodGet, "https://app.test")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.test" {
		t.Fatalf("ACAO = %q", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("ACAC = %q", got)
	}

	if w.Code != http.StatusTeapot {
		t.Fatalf("handler not reached, code = %d", w.Code)
	}

	// Untrusted origin: no ACAO, handler still runs (browser blocks read).
	w = callCORS(t, cfg, http.MethodGet, "https://evil.test")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("untrusted origin echoed: %q", got)
	}

	if w.Code != http.StatusTeapot {
		t.Fatalf("handler not reached, code = %d", w.Code)
	}

	// Preflight on trusted origin: 204 + headers, handler skipped.
	w = callCORS(t, cfg, http.MethodOptions, "https://app.test")
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight code = %d", w.Code)
	}

	if got := w.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Fatalf("max-age = %q", got)
	}

	if h := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(h, "X-B3-Traceid") {
		t.Fatalf("allow-headers missing tracing headers: %q", h)
	}
}

func TestCORSMiddlewareDenyByDefault(t *testing.T) {
	cfg := (&CORSConfig{}).Ensure()
	w := callCORS(t, cfg, http.MethodGet, "https://any.test")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("default must deny, ACAO = %q", got)
	}
}

func TestStatusMiddleware(t *testing.T) {
	var got int
	h := NewStatusMiddleware()(func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusNotFound)
		got = StatusFromContext(r.Context())

		return nil
	})
	w := httptest.NewRecorder()
	if err := h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/", http.NoBody))); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	if got != http.StatusNotFound {
		t.Fatalf("recorded status = %d", got)
	}

	// Implicit 200 via Write without WriteHeader.
	h2 := NewStatusMiddleware()(func(w http.ResponseWriter, r bunrouter.Request) error {
		_, _ = w.Write([]byte("hi"))
		got = StatusFromContext(r.Context())

		return nil
	})
	if err := h2(httptest.NewRecorder(), bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/", http.NoBody))); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	if got != http.StatusOK {
		t.Fatalf("implicit status = %d", got)
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	h := NewBodyLimitMiddleware(4)(func(w http.ResponseWriter, r bunrouter.Request) error {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected body-too-large error")
		}

		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	if err := h(httptest.NewRecorder(), bunrouter.NewRequest(req)); err != nil {
		t.Fatalf("middleware error: %v", err)
	}
}

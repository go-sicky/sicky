package sicky

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-sicky/sicky/infra"
)

var errTestBoom = errors.New("boom-test-failure")

func TestGuardNoTokenLoopbackOnly(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/services", http.NoBody)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("external without token want 403 got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/services", http.NoBody)
	req2.RemoteAddr = "127.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("loopback without token want 200 got %d", rec2.Code)
	}
}

func TestHealthCustomCheckers(t *testing.T) {
	m := NewManager(nil, "app", "v1")
	RegisterHealthChecker("biz-ok", func(ctx context.Context) error { return nil })
	RegisterHealthChecker("biz-bad", func(ctx context.Context) error { return errTestBoom })
	defer UnregisterHealthChecker("biz-ok")
	defer UnregisterHealthChecker("biz-bad")

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	m.health().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failing checker must degrade to 503, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "biz-ok") || !strings.Contains(body, "biz-bad") {
		t.Fatalf("custom checkers missing in body: %s", body)
	}

	if strings.Contains(body, "boom-test-failure") {
		t.Fatalf("backend error text must be redacted: %s", body)
	}

	liveReq := httptest.NewRequest(http.MethodGet, "/live", http.NoBody)
	liveRec := httptest.NewRecorder()
	m.live().ServeHTTP(liveRec, liveReq)
	if liveRec.Code != http.StatusOK {
		t.Fatalf("live must be 200, got %d", liveRec.Code)
	}

	UnregisterHealthChecker("biz-bad")
	rec2 := httptest.NewRecorder()
	m.ready().ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/ready", http.NoBody))
	if rec2.Code != http.StatusOK {
		t.Fatalf("ready must recover to 200 after unregister, got %d", rec2.Code)
	}
}

func TestManagerTLSValidate(t *testing.T) {
	half := &ManagerConfig{Address: "127.0.0.1:0", TLSCertPEM: "cert"}
	if err := half.Ensure().Validate(); !errors.Is(err, ErrManagerIncompleteTLSConfig) {
		t.Fatalf("half-TLS must fail validation, got %v", err)
	}

	m := NewManager(half, "app", "v1")
	if err := m.Start(); !errors.Is(err, ErrManagerIncompleteTLSConfig) {
		_ = m.Stop()
		t.Fatalf("half-TLS Start must fail fast, got %v", err)
	}

	bogus := &ManagerConfig{Address: "127.0.0.1:0", TLSCertPEM: "cert", TLSKeyPEM: "key"}
	m2 := NewManager(bogus, "app", "v1")
	if err := m2.Start(); err == nil {
		_ = m2.Stop()
		t.Fatal("unparseable PEM Start must fail")
	}
}

func TestGuardBearer(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888", AuthToken: "s3cret"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	req := httptest.NewRequest(http.MethodGet, "/config", http.NoBody)
	req.RemoteAddr = "8.8.8.8:1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer want 401 got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/config", http.NoBody)
	req2.RemoteAddr = "8.8.8.8:1"
	req2.Header.Set("Authorization", "Bearer s3cret")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("valid bearer want 200 got %d", rec2.Code)
	}
}

func TestSanitizeRedactsSecrets(t *testing.T) {
	if got := sanitizeValue("dsn", "postgres://u:p@h/db"); got != "***redacted***" {
		t.Fatalf("dsn not redacted: %v", got)
	}

	if got := sanitizeValue("addr", "1.2.3.4"); got != "1.2.3.4" {
		t.Fatal("non-secret over-redacted")
	}

	// Connection-string keys commonly embed userinfo and must redact.
	for _, k := range []string{"uri", "url", "broker", "addresses", "cloud_id", "creds_file", "nkey_file", "ca_cert_file", "root_ca_file"} {
		if got := sanitizeValue(k, "nats://u:p@h:4222"); got != "***redacted***" {
			t.Fatalf("%s not redacted: %v", k, got)
		}
	}

	// Plain usernames stay visible for debugging.
	if got := sanitizeValue("username", "ops"); got != "ops" {
		t.Fatalf("username over-redacted: %v", got)
	}

	m := NewManager(nil, "app", "v1")
	m.cfgVar = &Config{
		Infra: &InfraConfig{
			Redis: &infra.RedisConfig{Addr: "h:6379", Password: "pw"},
		},
	}

	raw, _ := json.Marshal(m.sanitizedConfig())
	if strings.Contains(string(raw), "pw") {
		t.Fatalf("password leaked in sanitized config: %s", raw)
	}

	if !strings.Contains(string(raw), "h:6379") {
		t.Fatalf("non-secret lost in sanitized config: %s", raw)
	}
}

func TestHealthNoInfraIsHealthy(t *testing.T) {
	m := NewManager(nil, "app", "v1")
	cs := m.collectComponentHealth(context.Background())
	if len(cs) == 0 {
		t.Fatal("no components")
	}

	for _, c := range cs {
		if c.Status == "unhealthy" {
			t.Fatalf("empty infra should not be unhealthy: %+v", c)
		}
	}
}

func TestValidateClamp(t *testing.T) {
	cfg := &Config{
		LogLevel: "verbose",
		Manager: &ManagerConfig{
			Enable:          true,
			ShutdownTimeout: -3,
			ReadTimeout:     -1,
			WriteTimeout:    -1,
			IdleTimeout:     -1,
		},
		Tracer: &TracerConfig{Type: "grpc", Timeout: -5, SampleRate: 9},
	}

	validateConfig(cfg)
	if cfg.Manager.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("shutdown not clamped: %d", cfg.Manager.ShutdownTimeout)
	}

	if cfg.Manager.ReadTimeout != DefaultManagerReadTimeout ||
		cfg.Manager.WriteTimeout != DefaultManagerWriteTimeout ||
		cfg.Manager.IdleTimeout != DefaultManagerIdleTimeout {
		t.Fatal("http timeouts not clamped")
	}

	if cfg.Tracer.Timeout != 0 || cfg.Tracer.SampleRate != 1.0 {
		t.Fatal("tracer not clamped")
	}
}

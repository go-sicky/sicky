package sicky

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-sicky/sicky/infra"
)

func TestGuardNoTokenLoopbackOnly(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	req := httptest.NewRequest("GET", "/services", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("external without token want 403 got %d", rec.Code)
	}

	req2 := httptest.NewRequest("GET", "/services", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("loopback without token want 200 got %d", rec2.Code)
	}
}

func TestGuardBearer(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888", AuthToken: "s3cret"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	req := httptest.NewRequest("GET", "/config", nil)
	req.RemoteAddr = "8.8.8.8:1"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no bearer want 401 got %d", rec.Code)
	}

	req2 := httptest.NewRequest("GET", "/config", nil)
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

package sicky

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	// httptest defaults the Host to example.com; a local client asks for
	// a loopback name, which is what the guard now checks as well.
	req2.Host = "localhost"
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
			Enable:          new(true),
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

func TestSanitizeRedactsPEMAndEndpoint(t *testing.T) {
	for _, k := range []string{"tls_key_pem", "tls_cert_pem", "endpoint", "cloud_url", "key_pem", "cert_pem", "privatekey"} {
		if got := sanitizeValue(k, "-----BEGIN PRIVATE KEY-----secret"); got != "***redacted***" {
			t.Fatalf("%s not redacted: %v", k, got)
		}
	}

	m := NewManager(nil, "app", "v1")
	m.cfgVar = &Config{
		Manager: &ManagerConfig{
			TLSCertPEM: "CERT",
			TLSKeyPEM:  "KEY-PRIVATE-MATERIAL",
		},
	}

	raw, _ := json.Marshal(m.sanitizedConfig())
	if strings.Contains(string(raw), "KEY-PRIVATE-MATERIAL") {
		t.Fatalf("tls private key leaked in sanitized config: %s", raw)
	}
}

func TestManagerPathValidation(t *testing.T) {
	bad := []*ManagerConfig{
		{Address: "127.0.0.1:0", MetricsPath: "metrics"},
		{Address: "127.0.0.1:0", HealthPath: "/metrics"},
		{Address: "127.0.0.1:0", LivePath: "/health"},
	}

	for i, c := range bad {
		if err := c.Ensure().Validate(); err == nil {
			t.Errorf("case %d: invalid path must fail validation", i)
		}
	}

	ok := (&ManagerConfig{Address: "127.0.0.1:0", MetricsPath: "/m"}).Ensure()
	if err := ok.Validate(); err != nil {
		t.Errorf("valid paths must pass: %v", err)
	}

	// Invalid path must fail Start fast instead of panicking the process.
	m := NewManager(&ManagerConfig{Address: "127.0.0.1:0", HealthPath: "health"}, "app", "v1")
	if err := m.Start(); err == nil {
		_ = m.Stop()
		t.Fatal("invalid path Start must fail")
	}
}

func TestManagerBindFailureReturnsError(t *testing.T) {
	m1 := NewManager(&ManagerConfig{Address: "127.0.0.1:0"}, "app", "v1")
	if err := m1.Start(); err != nil {
		t.Fatalf("first bind failed: %v", err)
	}

	defer func() { _ = m1.Stop() }()
	addr := m1.Addr()

	m2 := NewManager(&ManagerConfig{Address: addr}, "app2", "v2")
	if err := m2.Start(); err == nil {
		_ = m2.Stop()
		t.Fatal("conflicting bind must return an error")
	}
}

func TestManagerStartStopRestart(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: "127.0.0.1:0"}, "app", "v1")
	for i := range 3 {
		if err := m.Start(); err != nil {
			t.Fatalf("start %d failed: %v", i, err)
		}

		if err := m.Stop(); err != nil {
			t.Fatalf("stop %d failed: %v", i, err)
		}
	}

	if err := m.Stop(); err != nil {
		t.Fatal("double stop must be nil")
	}
}

// TestManagerConcurrentStartStopAndConfig exercises both halves of the
// Start/Stop race: running was published before wg.Add (a Stop landing in
// between saw a zero counter and returned while Start then Added into a
// finished Wait), and /config read m.config without the lock while a
// restart reassigned it.
func TestManagerConcurrentStartStopAndConfig(t *testing.T) {
	m := NewManager(&ManagerConfig{
		Address:      "127.0.0.1:0",
		ExposeConfig: true,
		AuthToken:    "t0ken",
	}, "app", "v1")

	stop := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() {
		for i := range 20 {
			if err := m.Start(); err != nil {
				t.Errorf("start %d: %v", i, err)

				return
			}

			if err := m.Stop(); err != nil {
				t.Errorf("stop %d: %v", i, err)

				return
			}
		}
	})

	wg.Go(func() {
		handler := m.cfg()
		for {
			select {
			case <-stop:
				return
			default:
				req := httptest.NewRequest(http.MethodGet, "/config", http.NoBody)
				req.RemoteAddr = "127.0.0.1:12345"
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			}
		}
	})

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Leave a clean state behind for other tests.
	_ = m.Stop()
}

// TestGuardRejectsReboundHost: a page on an attacker's domain can re-bind
// its hostname to 127.0.0.1, and the peer check alone cannot tell that
// from a local process - the Host header can.
func TestGuardRejectsReboundHost(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tt := range []struct {
		host string
		want int
	}{
		{"localhost:8888", http.StatusOK},
		{"127.0.0.1:8888", http.StatusOK},
		{"[::1]:8888", http.StatusOK},
		{"", http.StatusOK}, // browsers always send one; tests may not
		{"evil.example:8888", http.StatusForbidden},
		{"attacker.example", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodGet, "/services", http.NoBody)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Host = tt.host

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != tt.want {
			t.Errorf("Host %q: status = %d, want %d", tt.host, rec.Code, tt.want)
		}
	}
}

// TestGuardAcceptsConfiguredAndAllowedHosts: a manager reached through
// its own address, or through a reverse proxy listed in allowed_hosts,
// keeps working without a token.
func TestGuardAcceptsConfiguredAndAllowedHosts(t *testing.T) {
	m := NewManager(&ManagerConfig{
		Address:      "10.1.2.3:8888",
		AllowedHosts: []string{"api.internal:8888"},
	}, "app", "v1")

	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, host := range []string{"10.1.2.3:8888", "api.internal:8888", "API.Internal"} {
		req := httptest.NewRequest(http.MethodGet, "/services", http.NoBody)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Host = host

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Host %q: status = %d, want 200", host, rec.Code)
		}
	}
}

// TestGuardTokenSkipsHostCheck: an authenticated caller is not subject
// to the Host allowlist - the token is the boundary there.
func TestGuardTokenSkipsHostCheck(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888", AuthToken: "s3cret"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/services", http.NoBody)
	req.RemoteAddr = "192.0.2.10:4444"
	req.Host = "api.example.com"
	req.Header.Set("Authorization", "Bearer s3cret")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("token + foreign host: status = %d, want 200", rec.Code)
	}
}

// TestRunCheckTimesOutIgnoringContext: a checker that never returns must
// not pin /health for every caller.
func TestRunCheckTimesOutIgnoringContext(t *testing.T) {
	block := make(chan struct{})

	start := time.Now()
	err := runCheck(t.Context(), 50*time.Millisecond, func(context.Context) error {
		<-block

		return nil
	})

	elapsed := time.Since(start)
	close(block)

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runCheck = %v, want a deadline error", err)
	}

	if elapsed > 2*time.Second {
		t.Fatalf("runCheck took %s, want ~50ms", elapsed)
	}
}

func TestRunCheckPassesErrorThrough(t *testing.T) {
	boom := errors.New("backend down")
	err := runCheck(t.Context(), time.Second, func(context.Context) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("runCheck = %v, want the checker's error", err)
	}
}

// TestRunCheckRecoversPanic: business checkers are user code running on
// their own goroutine, where a panic would otherwise kill the process.
func TestRunCheckRecoversPanic(t *testing.T) {
	err := runCheck(t.Context(), time.Second, func(context.Context) error {
		panic("boom-in-checker")
	})

	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("runCheck = %v, want a panicked-check error", err)
	}
}

// TestGuardThrottlesRepeatedTokenFailures: the comparison is
// constant-time (flat cost per attempt), so online brute force has to be
// limited by attempt count instead.
func TestGuardThrottlesRepeatedTokenFailures(t *testing.T) {
	m := NewManager(&ManagerConfig{Address: ":8888", AuthToken: "s3cret"}, "app", "v1")
	h := m.guardSensitive(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	probe := func(auth string) int {
		req := httptest.NewRequest(http.MethodGet, "/config", http.NoBody)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Host = "localhost"
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	// Wrong tokens are refused until the window is exhausted...
	for i := range authMaxFailures {
		if got := probe("Bearer wrong"); got != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i, got)
		}
	}

	if got := probe("Bearer wrong"); got != http.StatusTooManyRequests {
		t.Fatalf("exhausted window: status = %d, want 429", got)
	}

	// ...and a correct token is refused too while it lasts: the counter
	// only clears on success or after the window.
	if got := probe("Bearer s3cret"); got != http.StatusTooManyRequests {
		t.Fatalf("blocked correct token: status = %d, want 429", got)
	}

	// The window expiry restores service without a restart.
	old := authWindow
	authWindow = 0
	t.Cleanup(func() { authWindow = old })

	if got := probe("Bearer s3cret"); got != http.StatusOK {
		t.Fatalf("after the window: status = %d, want 200", got)
	}

	// A successful authentication resets the count.
	m.auth.fail()
	m.auth.fail()
	if m.auth.blocked() {
		t.Fatal("a short count must not block")
	}

	probe("Bearer s3cret")
	if m.auth.blocked() {
		t.Fatal("a successful authentication must reset the counter")
	}
}

// TestDefaultManagerBindsLoopback: Run(nil) hands out this default, so
// a wildcard bind would publish the manager on every interface without
// anyone asking for it.
func TestDefaultManagerBindsLoopback(t *testing.T) {
	cfg := DefaultManagerConfig().Ensure()
	if cfg.Address != "127.0.0.1:8888" {
		t.Fatalf("default manager address = %q, want 127.0.0.1:8888", cfg.Address)
	}

	if isExternalListen(cfg.Address) {
		t.Fatal("the default must not count as an external listen")
	}

	if isExternalListen(":8888") != true {
		t.Fatal("a wildcard bind must still count as external")
	}
}

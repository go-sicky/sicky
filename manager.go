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
 * @file manager.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 12/16/2024
 */

package sicky

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
)

type componentHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthCheck probes one business component. A nil error means healthy;
// any error means unhealthy (the text is redacted on the wire, details
// go to the server log).
type HealthCheck func(ctx context.Context) error

var (
	healthCheckers   = make(map[string]HealthCheck)
	healthCheckersMu sync.RWMutex
)

// RegisterHealthChecker adds a business health check merged into the
// /health and /ready component lists. Names must be unique; re-registering
// replaces the previous check. Unregister with UnregisterHealthChecker.
func RegisterHealthChecker(name string, check HealthCheck) {
	if name == "" || check == nil {
		return
	}

	healthCheckersMu.Lock()
	defer healthCheckersMu.Unlock()

	healthCheckers[name] = check
}

// UnregisterHealthChecker removes a business health check.
func UnregisterHealthChecker(name string) {
	healthCheckersMu.Lock()
	defer healthCheckersMu.Unlock()

	delete(healthCheckers, name)
}

func snapshotHealthCheckers() map[string]HealthCheck {
	healthCheckersMu.RLock()
	defer healthCheckersMu.RUnlock()

	out := make(map[string]HealthCheck, len(healthCheckers))
	maps.Copy(out, healthCheckers)

	return out
}

// Manager is a sicky component.
type Manager struct {
	ctx     context.Context
	config  *ManagerConfig
	srv     *http.Server
	running bool

	cfgVar     *Config
	appName    string
	appVersion string

	metricsRegistry *prometheus.Registry

	sync.RWMutex
	wg sync.WaitGroup
}

// NewManager creates a new Manager.
func NewManager(cfg *ManagerConfig, appName, appVersion string) *Manager {
	if cfg == nil {
		cfg = DefaultManagerConfig()
	} else {
		cfg = cfg.Ensure()
	}

	m := &Manager{
		ctx:        context.Background(),
		config:     cfg,
		appName:    appName,
		appVersion: appVersion,
	}

	m.metricsRegistry = prometheus.NewRegistry()
	cs := slices.Collect(maps.Values(metrics.GetAll()))
	m.metricsRegistry.MustRegister(cs...)

	return m
}

// Context returns the component context.
func (m *Manager) Context() context.Context {
	return m.ctx
}

// Server is part of the public API.
func (m *Manager) Server() *http.Server {
	return m.srv
}

// Addr returns the address.
func (m *Manager) Addr() string {
	m.RLock()
	srv := m.srv
	cfg := m.config
	m.RUnlock()
	if srv == nil {
		return ""
	}

	return utils.Advertise(srv.Addr, cfg.AdvertiseAddress, "tcp").String()
}

// Port returns the port.
func (m *Manager) Port() int {
	m.RLock()
	srv := m.srv
	m.RUnlock()
	if srv == nil {
		return 0
	}

	_, port, _ := net.SplitHostPort(srv.Addr)
	portV, _ := strconv.Atoi(port)

	return portV
}

// Start starts the component.
func (m *Manager) Start() error {
	m.Lock()
	if m.running {
		m.Unlock()

		return nil
	}

	if m.config == nil {
		m.config = DefaultManagerConfig()
	} else {
		m.config = m.config.Ensure()
	}

	cfg := m.config
	m.Unlock()

	// A half-configured TLS must never silently serve plaintext, and
	// config-driven paths must never panic the mux registration.
	if err := cfg.Validate(); err != nil {
		logger.Logger.ErrorContext(
			m.ctx,
			"manager configuration invalid",
			"error", err.Error(),
		)

		return err
	}

	srv := &http.Server{
		Addr:              cfg.Address,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.IdleTimeout) * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// servePlaintext reports whether the listener runs without TLS.
	servePlaintext := true
	if cfg.TLSCertPEM != "" && cfg.TLSKeyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(cfg.TLSCertPEM), []byte(cfg.TLSKeyPEM))
		if err != nil {
			logger.Logger.ErrorContext(
				m.ctx,
				"manager TLS certification failed",
				"error", err.Error(),
			)

			return err
		}

		srv.TLSConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}

		servePlaintext = false
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, m.metrics())
	mux.Handle(cfg.HealthPath, m.health())
	mux.Handle(cfg.LivePath, m.live())
	mux.Handle(cfg.ReadyPath, m.ready())
	mux.Handle(cfg.VersionPath, m.version())
	mux.Handle(cfg.InfoPath, m.info())
	mux.Handle(cfg.ConfigPath, m.guardSensitive(m.cfg()))
	mux.Handle(cfg.ServicePoolPath, m.guardSensitive(m.servicePool()))
	srv.Handler = mux

	// Bind synchronously so a port conflict is a real Start error instead
	// of a log-only failure the orchestrator never learns about.
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		logger.Logger.ErrorContext(
			m.ctx,
			"manager listen failed",
			"address", cfg.Address,
			"error", err.Error(),
		)

		return err
	}

	srv.Addr = listener.Addr().String()

	// Publish server + running atomically so a concurrent Stop can never
	// orphan the listener (Stop either sees running=false and returns, or
	// sees the full state and shuts it down).
	m.Lock()
	if m.running {
		// A concurrent Start raced in and won; discard this listener.
		m.Unlock()
		_ = listener.Close()

		return nil
	}

	m.srv = srv
	m.running = true
	m.Unlock()

	if cfg.AuthToken == "" && isExternalListen(cfg.Address) {
		logger.Logger.WarnContext(
			m.ctx,
			"manager listens on external address without AuthToken; /config and /services are loopback-only",
			"address", cfg.Address,
		)
	}

	m.wg.Add(1)
	go func(s *http.Server, l net.Listener, useTLS bool) {
		defer m.wg.Done()

		var err error
		if useTLS {
			// Certificates are already loaded into s.TLSConfig;
			// empty cert/key file names keep ServeTLS from touching disk.
			err = s.ServeTLS(l, "", "")
		} else {
			err = s.Serve(l)
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.ErrorContext(
				m.ctx,
				"manager server listen failed",
				"error", err.Error(),
			)
			m.Lock()
			m.running = false
			m.Unlock()

			return
		}

		logger.Logger.InfoContext(
			m.ctx,
			"manager server closed",
		)
	}(srv, listener, !servePlaintext)

	logger.Logger.InfoContext(
		m.ctx,
		"manager server started",
		"address", m.Addr(),
		"port", m.Port(),
		"tls", !servePlaintext,
	)

	return nil
}

// Stop stops the component and releases resources.
func (m *Manager) Stop() error {
	m.Lock()
	if !m.running {
		m.Unlock()

		return nil
	}

	srv := m.srv
	cfg := m.config
	m.Unlock()

	if srv == nil {
		m.Lock()
		m.running = false
		m.Unlock()

		return nil
	}

	timeout := 5 * time.Second
	if cfg != nil && cfg.ShutdownTimeout > 0 {
		timeout = time.Duration(cfg.ShutdownTimeout) * time.Second
	}

	// Defensive: never block forever on a cancelless context.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	serr := srv.Shutdown(ctx)
	cancel()
	if serr != nil && !errors.Is(serr, http.ErrServerClosed) {
		// Force-close the listener and remaining connections so the
		// serve goroutine and the wait below always terminate.
		_ = srv.Close()
	}

	// Bound the wait: a stuck handler must never wedge shutdown forever.
	waitDone := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(timeout + time.Second):
		serr = errors.Join(serr, errors.New("manager shutdown timed out"))
	}

	logger.Logger.InfoContext(
		m.ctx,
		"manager server shutdown",
	)

	m.Lock()
	m.running = false
	m.Unlock()

	if serr != nil && !errors.Is(serr, http.ErrServerClosed) {
		return serr
	}

	return nil
}

/* {{{ [Manager] */
// guardSensitive enforces Bearer auth on debug/topology endpoints
// (/config, /services) when AuthToken is set. Without a token, only
// loopback clients may reach them, so a default external bind (:8888)
// does not leak config or topology. /metrics stays public.
func (m *Manager) guardSensitive(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.RLock()
		token := ""
		if m.config != nil {
			token = m.config.AuthToken
		}

		m.RUnlock()
		if token != "" {
			got := r.Header.Get("Authorization")
			want := "Bearer " + token
			// Compare equal-length digests so a length mismatch does not
			// short-circuit into a timing oracle on the token length.
			gotSum := sha256.Sum256([]byte(got))
			wantSum := sha256.Sum256([]byte(want))
			if subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) != 1 {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			next.ServeHTTP(w, r)

			return
		}

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		ip := net.ParseIP(strings.TrimSpace(host))
		if ip == nil || !ip.IsLoopback() {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func isExternalListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// ":8888" style or unparsable: treat bare ":port" as external.
		return strings.HasPrefix(strings.TrimSpace(addr), ":")
	}

	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}

	ip := net.ParseIP(host)

	return ip == nil || !ip.IsLoopback()
}

// sanitizeValue redacts secret-looking keys/values before /config exposure.
// Connection-string keys (uri/url/broker/addresses/...) are redacted because
// they commonly embed userinfo; plain usernames/client IDs are left visible
// for debugging (/config itself is already auth-or-loopback gated).
func sanitizeValue(key string, val any) any {
	lk := strings.ToLower(key)
	for _, sub := range []string{"dsn", "password", "passwd", "pwd", "secret", "token", "apikey", "api_key", "api-key", "auth", "private_key", "privatekey", "accesskey", "access_key", "secret_key", "session_token", "uri", "url", "broker", "addresses", "cloud_id", "cloud_url", "creds_file", "nkey_file", "ca_file", "ca_cert_file", "root_ca_file", "tls_key", "tls_cert", "key_pem", "cert_pem", "private_key_pem", "endpoint"} {
		if strings.Contains(lk, sub) {
			if s, ok := val.(string); ok && s != "" {
				return "***redacted***"
			}

			if val != nil {
				return "***redacted***"
			}

			return val
		}
	}

	switch v := val.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, vv := range v {
			out[k] = sanitizeValue(k, vv)
		}

		return out
	case []any:
		out := make([]any, len(v))
		for i, vv := range v {
			out[i] = sanitizeValue(key, vv)
		}

		return out
	default:
		return val
	}
}

func (m *Manager) sanitizedConfig() any {
	if m.cfgVar == nil {
		return nil
	}

	raw, err := json.Marshal(m.cfgVar)
	if err != nil {
		return map[string]any{errorField: "config marshal failed"}
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{errorField: "config unmarshal failed"}
	}

	return sanitizeValue("", decoded)
}

func (m *Manager) metrics() http.Handler {
	return promhttp.HandlerFor(
		m.metricsRegistry,
		promhttp.HandlerOpts{
			Registry: m.metricsRegistry,
		},
	)
}

func (m *Manager) health() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.writeHealth(w, m.collectComponentHealth(r.Context()))
	})
}

// live reports process liveness only: no dependency probes, always 200
// while the manager itself serves. Use it for orchestrator liveness;
// use /ready (or /health) for readiness.
func (m *Manager) live() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": statusHealthy, "version": m.appVersion})
	})
}

// ready aggregates the same component list as /health: only real
// failures degrade it to 503.
func (m *Manager) ready() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.writeHealth(w, m.collectComponentHealth(r.Context()))
	})
}

func (m *Manager) writeHealth(w http.ResponseWriter, components []componentHealth) {
	type status struct {
		Status     string            `json:"status"`
		Version    string            `json:"version"`
		Components []componentHealth `json:"components"`
	}

	// Only real failures degrade overall status. "not_configured"
	// components are reported but do not fail readiness.
	overall := statusHealthy
	for _, c := range components {
		if c.Status == statusUnhealthy {
			overall = "degraded"
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if overall == "degraded" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(
		&status{
			Status:     overall,
			Version:    m.appVersion,
			Components: components,
		},
	)
}

// Health status literals and well-known component names shared by the
// collector, the check dispatch switch, and the JSON envelopes.
const (
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"

	// errorField is the JSON key for error envelopes (kept identical to
	// the slog "error" field name by convention).
	errorField = "error"

	componentRedis      = "redis"
	componentBun        = "bun"
	componentClickhouse = "clickhouse"
	componentMongo      = "mongo"
	componentElastic    = "elastic"
	componentS3         = "s3"
	componentRistretto  = "ristretto"
	componentBadger     = "badger"
	componentNATS       = "nats"
	componentMQTT       = "mqtt"
)

func (m *Manager) collectComponentHealth(reqCtx context.Context) []componentHealth {
	ctx, cancel := context.WithTimeout(reqCtx, 2*time.Second)
	defer cancel()

	type checkDef struct {
		name string
		ping func(ctx context.Context) error
		// local reports configured-but-local-only state (no remote ping).
		local func() (configured bool, unhealthy bool)
	}

	defs := []checkDef{
		{name: componentRedis, ping: func(ctx context.Context) error {
			rdb := infra.GetRedis()
			if rdb == nil {
				return nil
			}

			return rdb.Ping(ctx).Err()
		}},
		{name: componentBun, ping: func(ctx context.Context) error {
			db := infra.GetBun()
			if db == nil {
				return nil
			}

			return db.PingContext(ctx)
		}},
		{name: componentClickhouse, ping: func(ctx context.Context) error {
			ch := infra.GetClickHouse()
			if ch == nil {
				return nil
			}

			return ch.Ping(ctx)
		}},
		{name: componentMongo, ping: func(ctx context.Context) error {
			mg := infra.GetMongo()
			if mg == nil {
				return nil
			}

			return mg.Ping(ctx, readpref.Primary())
		}},
		{name: componentElastic, ping: infra.PingElastic},
		{name: componentS3, ping: infra.PingS3},
		{name: componentRistretto, local: func() (bool, bool) { return infra.GetRistretto() != nil, false }},
		{name: componentBadger, local: func() (bool, bool) { return infra.GetBadger() != nil, false }},
		{name: componentNATS, local: func() (bool, bool) {
			nc := infra.GetNATS()
			if nc == nil {
				return false, false
			}

			return true, !nc.IsConnected()
		}},
		{name: componentMQTT, local: func() (bool, bool) {
			mc := infra.GetMQTT()
			if mc == nil {
				return false, false
			}

			return true, !mc.IsConnected()
		}},
	}

	// Snapshot business checkers once: sizing + fill share the same snapshot
	// so a concurrent Register/Unregister cannot skew the slice bounds.
	bizChecks := snapshotHealthCheckers()
	cs := make([]componentHealth, len(defs)+len(bizChecks))
	var wg sync.WaitGroup
	for i, d := range defs {
		wg.Add(1)
		go func(i int, d checkDef) {
			defer wg.Done()
			ch := componentHealth{Name: d.name, Status: "not_configured"}
			if d.ping != nil {
				// Determine configured state first without blocking.
				configured := true
				switch d.name {
				case componentRedis:
					configured = infra.GetRedis() != nil
				case componentBun:
					configured = infra.GetBun() != nil
				case componentClickhouse:
					configured = infra.GetClickHouse() != nil
				case componentMongo:
					configured = infra.GetMongo() != nil
				case componentElastic:
					configured = infra.GetElastic() != nil
				case componentS3:
					configured = infra.GetS3() != nil
				}

				if !configured {
					cs[i] = ch

					return
				}

				start := time.Now()
				derr := d.ping(ctx)
				metrics.ManagerHealthCheckDuration.WithLabelValues(d.name).Observe(time.Since(start).Seconds())
				if err := derr; err != nil {
					ch.Status = statusUnhealthy
					// Never expose backend error text on the unauthenticated
					// /health endpoint (it leaks addresses/auth details).
					// The detail goes to the server log only.
					logger.Logger.ErrorContext(
						ctx,
						"health check failed",
						"component", d.name,
						"error", err.Error(),
					)
					ch.Error = statusUnhealthy
				} else {
					ch.Status = statusHealthy
				}

				cs[i] = ch

				return
			}

			if d.local != nil {
				configured, unhealthy := d.local()
				if !configured {
					cs[i] = ch

					return
				}

				ch.Status = statusHealthy
				if unhealthy {
					ch.Status = statusUnhealthy
					ch.Error = "disconnected"
				}

				cs[i] = ch
			}
		}(i, d)
	}

	wg.Wait()

	// Business checkers registered via RegisterHealthChecker run under the
	// same timeout and join the component list after the infra checks.
	j := len(defs)
	for name, check := range bizChecks {
		ch := componentHealth{Name: name, Status: statusHealthy}
		start := time.Now()
		cerr := check(ctx)
		metrics.ManagerHealthCheckDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
		if err := cerr; err != nil {
			ch.Status = statusUnhealthy
			ch.Error = statusUnhealthy
			logger.Logger.ErrorContext(
				ctx,
				"health check failed",
				"component", name,
				"error", err.Error(),
			)
		}

		cs[j] = ch
		j++
	}

	return cs
}

func (m *Manager) version() http.Handler {
	type version struct {
		Version string `json:"version"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(
			&version{
				Version: m.appVersion,
			},
		)
	})
}

func (m *Manager) info() http.Handler {
	type info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(
			&info{
				Name:    m.appName,
				Version: m.appVersion,
			},
		)
	})
}

func (m *Manager) cfg() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.ExposeConfig {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m.sanitizedConfig())
	})
}

func (m *Manager) servicePool() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(registry.GetPool()); err != nil {
			// A marshal failure (e.g. a non-serializable instance) must
			// not silently produce an empty body.
			logger.Logger.ErrorContext(
				m.ctx,
				"service pool marshal failed",
				"error", err.Error(),
			)
		}
	})
}

/* }}} */

var managerApp *Manager

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

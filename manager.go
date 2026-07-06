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
	"encoding/json"
	"errors"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type componentHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

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

func NewManager(cfg *ManagerConfig, appName, appVersion string) *Manager {
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

func (m *Manager) Context() context.Context {
	return m.ctx
}

func (m *Manager) Server() *http.Server {
	return m.srv
}

func (m *Manager) Addr() string {
	if m.srv == nil {
		return ""
	}

	return utils.Advertise(m.srv.Addr, m.config.AdvertiseAddress, "tcp").String()
}

func (m *Manager) Port() int {
	if m.srv == nil {
		return 0
	}

	_, port, _ := net.SplitHostPort(m.srv.Addr)
	portV, _ := strconv.Atoi(port)

	return portV
}

func (m *Manager) Start() error {
	m.Lock()
	defer m.Unlock()

	if m.running {
		return nil
	}

	m.srv = &http.Server{Addr: m.config.Address}
	mux := http.NewServeMux()
	mux.Handle(m.config.MetricsPath, m.metrics())
	mux.Handle(m.config.HealthPath, m.health())
	mux.Handle(m.config.VersionPath, m.version())
	mux.Handle(m.config.InfoPath, m.info())
	mux.Handle(m.config.ConfigPath, m.cfg())
	mux.Handle(m.config.ServicePoolPath, m.servicePool())
	m.srv.Handler = mux
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		err := m.srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.ErrorContext(
				m.ctx,
				"Manager server listen failed",
				"error", err.Error(),
			)
			m.Lock()
			m.running = false
			m.Unlock()

			return
		}

		logger.Logger.InfoContext(
			m.ctx,
			"Manager server closed",
		)
	}()

	logger.Logger.InfoContext(
		m.ctx,
		"Manager server started",
		"address", m.Addr(),
		"port", m.Port(),
	)

	m.running = true

	return nil
}

func (m *Manager) Stop() error {
	m.Lock()
	defer m.Unlock()

	if !m.running {
		return nil
	}

	if m.config.ShutdownTimeout > 0 {
		ctx, cancel := context.WithTimeout(m.ctx, time.Duration(m.config.ShutdownTimeout)*time.Second)
		defer cancel()
		m.srv.Shutdown(ctx)
	} else {
		m.srv.Shutdown(m.ctx)
	}
	m.wg.Wait()
	logger.Logger.InfoContext(
		m.ctx,
		"Manager server shutdown",
	)

	m.running = false

	return nil
}

/* {{{ [Manager] */
func (m *Manager) metrics() http.Handler {
	return promhttp.HandlerFor(
		m.metricsRegistry,
		promhttp.HandlerOpts{
			Registry: m.metricsRegistry,
		},
	)
}

func (m *Manager) health() http.Handler {
	type status struct {
		Status     string            `json:"status"`
		Version    string            `json:"version"`
		Components []componentHealth `json:"components"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		components := m.collectComponentHealth()
		overall := "healthy"
		for _, c := range components {
			if c.Status != "healthy" {
				overall = "degraded"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(
			&status{
				Status:     overall,
				Version:    m.appVersion,
				Components: components,
			},
		)
	})
}

func (m *Manager) collectComponentHealth() []componentHealth {
	var cs []componentHealth
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	check := func(name string, ok bool, err error) {
		ch := componentHealth{Name: name, Status: "healthy"}
		if !ok {
			ch.Status = "not_configured"
		}
		if err != nil {
			ch.Status = "unhealthy"
			ch.Error = err.Error()
		}
		cs = append(cs, ch)
	}

	check("redis", infra.Redis != nil, func() error {
		if infra.Redis != nil {
			return infra.Redis.Ping(ctx).Err()
		}
		return nil
	}())

	check("bun", infra.Bun != nil, func() error {
		if infra.Bun != nil {
			return infra.Bun.Ping()
		}
		return nil
	}())

	check("ristretto", infra.Ristretto != nil, nil)
	check("badger", infra.Badger != nil, nil)
	check("nats", infra.Nats != nil && infra.Nats.IsConnected(), nil)
	check("mqtt", infra.MQTT != nil && infra.MQTT.IsConnected(), nil)
	check("elastic", infra.Elastic != nil, nil)
	check("clickhouse", infra.Clickhouse != nil, func() error {
		if infra.Clickhouse != nil {
			return infra.Clickhouse.Ping(ctx)
		}
		return nil
	}())
	check("mongo", infra.Mongo != nil, func() error {
		if infra.Mongo != nil {
			return infra.Mongo.Ping(ctx, nil)
		}
		return nil
	}())
	check("s3", infra.S3 != nil, nil)

	return cs
}

func (m *Manager) version() http.Handler {
	type version struct {
		Version string `json:"version"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(
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
		json.NewEncoder(w).Encode(
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
		json.NewEncoder(w).Encode(m.cfgVar)
	})
}

func (m *Manager) servicePool() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registry.GetPool())
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

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
	"net/http"
	"slices"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Manager struct {
	ctx     context.Context
	config  *ManagerConfig
	srv     *http.Server
	running bool

	sync.RWMutex
	wg sync.WaitGroup
}

func NewManager(cfg *ManagerConfig) *Manager {
	return &Manager{
		ctx:    context.Background(),
		config: cfg,
	}
}

func (m *Manager) Context() context.Context {
	return m.ctx
}

func (m *Manager) Server() *http.Server {
	return m.srv
}

func (m *Manager) Start() error {
	m.Lock()
	defer m.Unlock()

	if m.running {
		return nil
	}

	m.srv = &http.Server{Addr: m.config.Addr}
	mux := http.NewServeMux()
	mux.Handle(m.config.MetricsPath, m.metrics())
	mux.Handle(m.config.HealthPath, m.health())
	mux.Handle(m.config.VersionPath, m.version())
	mux.Handle(m.config.InfoPath, m.info())
	m.srv.Handler = mux
	m.wg.Add(1)
	go func() error {
		err := m.srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Logger.ErrorContext(
				m.ctx,
				"Manager server listen failed",
				"error", err.Error(),
			)

			m.wg.Done()

			return err
		}

		logger.Logger.InfoContext(
			m.ctx,
			"Manager server closed",
		)

		m.wg.Done()

		return nil
	}()

	logger.Logger.InfoContext(
		m.ctx,
		"Manager server started",
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

	m.srv.Shutdown(m.ctx)
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
	metricsRegistry := prometheus.NewRegistry()
	cs := slices.Collect(maps.Values(metrics.GetAll()))
	metricsRegistry.MustRegister(cs...)

	return promhttp.HandlerFor(
		metricsRegistry,
		promhttp.HandlerOpts{
			Registry: metricsRegistry,
		},
	)
}

func (m *Manager) health() http.Handler {
	type status struct {
		Status     string `json:"status"`
		Version    string `json:"version"`
		Components []any  `json:"components"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(
			&status{
				Status: "healthy",
			},
		)
	})
}

func (m *Manager) version() http.Handler {
	type version struct {
		Version string `json:"version"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(
			&version{
				Version: "0.0.1",
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
				Name:    "sicky",
				Version: "0.0.1",
			},
		)
	})
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

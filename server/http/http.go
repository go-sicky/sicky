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
 * @file http.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 08/27/2025
 */

package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
	"github.com/uptrace/bunrouter"
	"go.opentelemetry.io/otel/trace"
)

// ErrIncompleteTLSConfig is returned when only one of TLSCertPEM/TLSKeyPEM
// is set. Falling back to plaintext silently would be a security hole, so
// startup fails fast instead.
var ErrIncompleteTLSConfig = errors.New("incomplete TLS configuration: both tls_cert_pem and tls_key_pem must be set")

/* {{{ [Server] */
type HTTPServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *http.Server
	router        *bunrouter.Router
	running       bool
	stopping      bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata
	// listener is the socket we created in Start. Stop closes it
	// directly as a backstop: Shutdown before Serve tracks the socket
	// would otherwise leave Serve blocked on Accept forever.
	listener net.Listener

	sync.RWMutex
	wg sync.WaitGroup
}

// New HTTP server (net/http)
func New(opts *server.Options, cfg *Config) *HTTPServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	var (
		addr          net.Addr
		advertiseAddr net.Addr
		err           error
	)

	// TCP default
	addr, err = net.ResolveTCPAddr(cfg.Network, cfg.Address)
	if err != nil {
		opts.Logger.Fatal(
			"Network address resolve failed",
			"string", cfg.Address,
			"error", err.Error(),
		)
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveTCPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"Network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)
		}
	} else {
		advertiseAddr = addr
	}

	srv := &HTTPServer{
		config:        cfg,
		ctx:           opts.Context,
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
	}

	app := &http.Server{
		Addr:              addr.String(),
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
	app.SetKeepAlivesEnabled(!cfg.DisableKeepAlive)

	srv.app = app
	var tr trace.Tracer
	if tracer.Default() != nil {
		tr = tracer.Default().Tracer(srv.Name())
	}
	// Fail closed on illegal CORS (wildcard + credentials): deny all
	// cross-origin requests rather than emitting the combination.
	if err := cfg.CORS.Ensure().Validate(); err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Invalid CORS configuration, denying all origins",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"error", err.Error(),
		)
		cfg.CORS = (&CORSConfig{}).Ensure()
	}
	srv.router = bunrouter.New(
		bunrouter.Use(NewRecoveryMiddleware(opts.Logger)),
		bunrouter.Use(NewCORSMiddleware(cfg.CORS)),
		bunrouter.Use(NewBodyLimitMiddleware(cfg.BodyLimit)),
		bunrouter.Use(NewPropagationMiddleware()),
		bunrouter.Use(NewMetadataMiddleware()),
		bunrouter.Use(NewTracerMiddleware(
			TracerConfig{
				Tracer: tr,
			},
		)),
		bunrouter.Use(NewStatusMiddleware()),
		bunrouter.Use(NewAccessLoggerMiddleware(
			AccessLoggerMiddlewareConfig{
				AccessLoggerConfig: cfg.AccessLogger,
				Logger:             opts.Logger,
			},
		)),
	)
	srv.options.Logger.InfoContext(
		srv.ctx,
		"HTTP server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
	)

	server.Set(srv)

	return srv
}

func (srv *HTTPServer) Context() context.Context {
	return srv.ctx
}

func (srv *HTTPServer) Options() *server.Options {
	return srv.options
}

func (srv *HTTPServer) String() string {
	return "http"
}

func (srv *HTTPServer) ID() uuid.UUID {
	return srv.options.ID
}

func (srv *HTTPServer) Name() string {
	return srv.options.Name
}

func (srv *HTTPServer) Start() error {
	var (
		listener net.Listener
		cert     tls.Certificate
		err      error
	)

	srv.Lock()
	defer srv.Unlock()

	if srv.running || srv.stopping {
		// running
		return nil
	}

	srv.options.RunBeforeStart()

	// A half-configured TLS must never silently fall back to plaintext.
	if (srv.config.TLSCertPEM != "") != (srv.config.TLSKeyPEM != "") {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"TLS configuration incomplete",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
		)

		return ErrIncompleteTLSConfig
	}

	// Try TLS
	if srv.config.TLSCertPEM != "" && srv.config.TLSKeyPEM != "" {
		cert, err = tls.X509KeyPair([]byte(srv.config.TLSCertPEM), []byte(srv.config.TLSKeyPEM))
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"TLS certification failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}

		listener, err = tls.Listen(
			srv.addr.Network(),
			srv.addr.String(),
			&tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			},
		)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Network listen with TLS certificate failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.addr.Network(),
			srv.addr.String(),
		)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Network listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}
	}

	srv.addr = listener.Addr()
	srv.listener = listener
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	srv.app.Handler = srv.router
	go func() {
		defer srv.wg.Done()

		err := srv.app.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"HTTP server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)
		} else {
			srv.options.Logger.InfoContext(
				srv.ctx,
				"HTTP server closed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"addr", srv.addr.String(),
			)
		}
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"HTTP server listening",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.running = true
	srv.options.RunAfterStart()

	return nil
}

func (srv *HTTPServer) Stop() error {
	// Check-and-flag under lock, then release: holding Lock across
	// Shutdown/Wait would starve all RLock readers for the whole drain.
	srv.Lock()
	if !srv.running || srv.stopping {
		// Not running
		srv.Unlock()

		return nil
	}
	srv.stopping = true
	srv.options.RunBeforeStop()
	app := srv.app
	timeout := srv.config.ShutdownTimeout
	srv.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var errs error
	if err := app.Shutdown(ctx); err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"HTTP server shutdown failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"error", err.Error(),
		)
		errs = errors.Join(errs, err)
	}
	// Backstop for the shutdown-vs-Serve registration race: closing our
	// own socket unblocks Accept even if Shutdown ran before Serve
	// tracked it. Double-close is harmless (logged at debug).
	if ln := srv.listener; ln != nil {
		if err := ln.Close(); err != nil {
			srv.options.Logger.DebugContext(
				srv.ctx,
				"HTTP listener close (already closed by shutdown)",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)
		}
	}
	srv.wg.Wait()

	srv.Lock()
	srv.running = false
	srv.stopping = false
	srv.Unlock()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"HTTP server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.options.RunAfterStop()

	return errs
}

func (srv *HTTPServer) Running() bool {
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

func (srv *HTTPServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

func (srv *HTTPServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *HTTPServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

func (srv *HTTPServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

func (srv *HTTPServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *HTTPServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

func (srv *HTTPServer) Metadata() utils.Metadata {
	// Snapshot: the map is written during Start while handlers may read
	// it concurrently; returning the live map would race.
	if srv.metadata == nil {
		return utils.NewMetadata()
	}

	return srv.metadata.Clone()
}

func (srv *HTTPServer) App() *http.Server {
	return srv.app
}

func (srv *HTTPServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		hdl.Register(srv.router)
		srv.options.Logger.DebugContext(
			srv.ctx,
			"HTTP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

/* }}} */

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	Register(*bunrouter.Router)
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

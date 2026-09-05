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
 * @file fiber.go
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package fiber

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
)

// ErrIncompleteTLSConfig is returned when only one of TLSCertPEM/TLSKeyPEM
// is set. Falling back to plaintext silently would be a security hole, so
// startup fails fast instead.
var ErrIncompleteTLSConfig = errors.New("incomplete TLS configuration: both tls_cert_pem and tls_key_pem must be set")

// ErrShutdownTimeout is returned when graceful shutdown exceeds
// ShutdownTimeout; draining continues in the background.
var ErrShutdownTimeout = errors.New("graceful shutdown timed out")

/* {{{ [Server] */

// FiberServer : Server definition.
type FiberServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *fiber.App
	running       bool
	stopping      bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata
	// listener is the socket we created in Start. Stop closes it
	// directly: relying solely on fasthttp shutdown races with Serve
	// registering the listener (shutdown first = Serve blocks forever).
	listener net.Listener

	sync.RWMutex
	wg sync.WaitGroup
}

// New HTTP server (go-fiber).
func New(opts *server.Options, cfg *Config) *FiberServer {
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

		return nil
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveTCPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"Network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)

			return nil
		}
	} else {
		advertiseAddr = addr
	}

	srv := &FiberServer{
		config:        cfg,
		ctx:           opts.Context,
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
	}

	// Set tracer
	var tr trace.Tracer
	if tracer.Default() != nil {
		tr = tracer.Default().Tracer(srv.Name())
	}

	app := fiber.New(
		fiber.Config{
			Prefork:               false,
			DisableStartupMessage: true,
			ServerHeader:          opts.Name,
			AppName:               opts.Name,
			Network:               cfg.Network,
			DisableKeepalive:      cfg.DisableKeepAlive,
			StrictRouting:         cfg.StrictRouting,
			CaseSensitive:         cfg.CaseSensitive,
			ETag:                  cfg.Etag,
			BodyLimit:             cfg.BodyLimit,
			Concurrency:           cfg.Concurrency,
			ReadBufferSize:        cfg.ReadBufferSize,
			WriteBufferSize:       cfg.WriteBufferSize,
			ReadTimeout:           cfg.ReadTimeout,
			WriteTimeout:          cfg.WriteTimeout,
			IdleTimeout:           cfg.IdleTimeout,
		},
	)

	if cfg.EnableStackTrace {
		app.Use(recover.New(
			recover.Config{
				EnableStackTrace: true,
			},
		))
	} else {
		app.Use(recover.New(
			recover.ConfigDefault,
		))
	}

	// The order of middlewares is important
	// Issue was resolved at dawn on the first day of 2025, thanks to the remote class reunion >_<!
	// CORS is deny-by-default: an empty whitelist skips the middleware
	// entirely instead of falling back to AllowOrigins "*". An illegal
	// combination (wildcard + credentials) fails closed the same way.
	corsCfg := cfg.CORS.Ensure()
	if err := corsCfg.Validate(); err != nil {
		opts.Logger.ErrorContext(
			opts.Context,
			"Invalid CORS configuration, denying all origins",
			"server", srv.String(),
			"id", opts.ID,
			"name", opts.Name,
			"error", err.Error(),
		)
		corsCfg = (&CORSConfig{}).Ensure()
	}

	corsMiddleware := func(c *fiber.Ctx) error {
		return c.Next()
	}

	if len(corsCfg.AllowedOrigins) > 0 {
		corsMiddleware = cors.New(cors.Config{
			AllowOrigins:     strings.Join(corsCfg.AllowedOrigins, ", "),
			AllowCredentials: corsCfg.AllowCredentials,
			MaxAge:           corsCfg.MaxAge,
		})
	}

	app.Use(
		corsMiddleware,
		NewPropagationMiddleware(),
		NewTracerMiddleware(
			TracerConfig{
				Tracer: tr,
			},
		),
		NewMetadataMiddleware(),
		NewAccessLoggerMiddleware(
			AccessLoggerMiddlewareConfig{
				Logger:             opts.Logger,
				AccessLoggerConfig: cfg.AccessLogger,
			},
		),
	)

	srv.app = app

	// Register swagger
	if cfg.EnableSwagger {
		srv.Handle(NewSwagger(
			cfg.SwaggerPageTitle,
			cfg.SwaggerValidatorURL,
		))
	}

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

// Context returns the component context.
func (srv *FiberServer) Context() context.Context {
	return srv.ctx
}

// Options returns the runtime options.
func (srv *FiberServer) Options() *server.Options {
	return srv.options
}

// String returns a human-readable name.
func (srv *FiberServer) String() string {
	return "fiber"
}

// ID returns the unique instance ID.
func (srv *FiberServer) ID() uuid.UUID {
	return srv.options.ID
}

// Name returns the component name.
func (srv *FiberServer) Name() string {
	return srv.options.Name
}

// Start starts the component.
func (srv *FiberServer) Start() error {
	var (
		listener net.Listener
		cert     tls.Certificate
		err      error
	)

	srv.RLock()
	running := srv.running
	stopping := srv.stopping
	srv.RUnlock()
	if running || stopping {
		// running
		return nil
	}

	// Hooks run unlocked: they may call accessors (Addr/Port/...) which
	// take the read lock and would self-deadlock under the write lock.
	srv.options.RunBeforeStart()

	srv.Lock()

	if srv.running || srv.stopping {
		srv.Unlock()

		// running
		return nil
	}

	// A half-configured TLS must never silently fall back to plaintext.
	if (srv.config.TLSCertPEM != "") != (srv.config.TLSKeyPEM != "") {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"TLS configuration incomplete",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
		)

		srv.Unlock()

		return ErrIncompleteTLSConfig
	}

	// Try TLS first
	if srv.config.TLSCertPEM != "" && srv.config.TLSKeyPEM != "" {
		cert, err = tls.X509KeyPair([]byte(srv.config.TLSCertPEM), []byte(srv.config.TLSKeyPEM))
		if err != nil {
			srv.Unlock()
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
			srv.Unlock()
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
			srv.Unlock()
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
	if srv.config.AdvertiseAddress == "" {
		srv.advertiseAddr = listener.Addr()
	}

	srv.listener = listener
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Go(func() {
		err := srv.app.Listener(listener)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"HTTP server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"HTTP server closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"addr", srv.addr.String(),
		)
	})

	srv.options.Logger.InfoContext(
		srv.ctx,
		"HTTP server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.running = true
	srv.Unlock()

	srv.options.RunAfterStart()

	return nil
}

// Stop stops the component and releases resources.
func (srv *FiberServer) Stop() error {
	// Check-and-flag under lock, then release: holding Lock across
	// Shutdown/Wait would starve all RLock readers for the whole drain.
	srv.Lock()
	if !srv.running || srv.stopping {
		// Not running
		srv.Unlock()

		return nil
	}

	srv.stopping = true
	app := srv.app
	timeout := srv.config.ShutdownTimeout
	srv.Unlock()

	// Hooks run unlocked: they may call accessors which take the read
	// lock and would self-deadlock under the write lock.
	srv.options.RunBeforeStop()

	// Use the fiber-level shutdown, then close our own listener as a
	// backstop: if Stop wins the race against Serve registering the
	// socket with fasthttp, shutdown alone never unblocks Accept.
	// Closing an already-closed listener only logs; double-close via
	// fasthttp + us is harmless (second Close returns an error we log).
	var errs error
	if err := app.ShutdownWithTimeout(timeout); err != nil && !isClosedConnError(err) {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Fiber server shutdown failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"error", err.Error(),
		)
		errs = errors.Join(errs, err)
		// fasthttp reports an exceeded ShutdownWithTimeout as
		// context.DeadlineExceeded; surface it as ErrShutdownTimeout
		// (mirrors the gRPC server) so callers can errors.Is on it.
		if errors.Is(err, context.DeadlineExceeded) {
			errs = errors.Join(errs, ErrShutdownTimeout)
		}
	}

	if ln := srv.listener; ln != nil {
		if err := ln.Close(); err != nil {
			srv.options.Logger.DebugContext(
				srv.ctx,
				"Fiber listener close (already closed by shutdown)",
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

// isClosedConnError reports benign double-close noise: our Stop backstop
// closes the socket outside fasthttp bookkeeping, so a later shutdown may
// re-close a stale s.ln entry. The socket is already down either way.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	return strings.Contains(err.Error(), "use of closed network connection")
}

// Running reports whether the component is running.
func (srv *FiberServer) Running() bool {
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

// Addr returns the address.
func (srv *FiberServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

// IP returns the IP.
func (srv *FiberServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// Port returns the port.
func (srv *FiberServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

// AdvertiseAddr returns the advertise address.
func (srv *FiberServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

// AdvertiseIP returns the advertise IP.
func (srv *FiberServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// AdvertisePort returns the advertise port.
func (srv *FiberServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

// Metadata returns the metadata.
func (srv *FiberServer) Metadata() utils.Metadata {
	// Snapshot under the read lock: the map is written during Start while
	// handlers may read it concurrently; returning the live map would race.
	srv.RLock()
	defer srv.RUnlock()

	if srv.metadata == nil {
		return utils.NewMetadata()
	}

	return srv.metadata.Clone()
}

// App returns the app.
func (srv *FiberServer) App() *fiber.App {
	return srv.app
}

// Handle registers handlers.
func (srv *FiberServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		hdl.Register(srv.app)
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

/* {{{ [Handler]. */
type Handler interface {
	Name() string
	Type() string
	Register(app *fiber.App)
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

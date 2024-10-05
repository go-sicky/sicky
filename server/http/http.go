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
 * @since 11/20/2023
 */

package http

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

/* {{{ [Server] */

// HTTPServer : Server definition
type HTTPServer struct {
	config   *Config
	ctx      context.Context
	options  *server.Options
	app      *fiber.App
	runing   bool
	addr     net.Addr
	metadata utils.Metadata

	sync.RWMutex
	wg sync.WaitGroup

	//tracer trace.Tracer
}

// New HTTP server (go-fiber)
func New(opts *server.Options, cfg *Config) *HTTPServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &HTTPServer{
		config:   cfg,
		ctx:      context.Background(),
		addr:     addr,
		runing:   false,
		options:  opts,
		metadata: utils.NewMetadata(),
	}

	// Set tracer
	// if srv.options.TraceProvider() != nil {
	// 	srv.tracer = srv.options.TraceProvider().Tracer(srv.Name() + "@" + srv.String())
	// }

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
	app.Use(
		cors.New(),
		NewPropagationMiddleware(),
		// tracer.NewFiberMiddleware(
		// 	&tracer.FiberMiddlewareConfig{
		// 		Tracer: srv.tracer,
		// 	},
		// ),
		//logger.NewFiberMiddleware(),
		//NewMetadataMiddleware(),
		NewAccessLoggerMiddleware(
			&AccessLoggerMiddlewareConfig{
				Logger:             opts.Logger,
				AccessLoggerConfig: cfg.AccessLogger,
			},
		),
	)

	srv.app = app

	// Register swagger
	if cfg.EnableSwagger {
		srv.Handle(NewSwagger())
	}

	srv.options.Logger.InfoContext(
		srv.ctx,
		"Server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
	)

	server.Instance(opts.ID, srv)

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

	if srv.runing {
		// Runing
		return nil
	}

	// Try TLS first
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
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Listener(listener)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"Server closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"addr", srv.addr.String(),
		)
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"Server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.runing = true

	return nil
}

func (srv *HTTPServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.runing {
		// Not runing
		return nil
	}

	srv.app.Server().Shutdown()
	srv.wg.Wait()
	srv.runing = false

	return nil
}

func (srv *HTTPServer) Addr() net.Addr {
	return srv.addr
}

func (srv *HTTPServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *HTTPServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *HTTPServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *HTTPServer) App() *fiber.App {
	return srv.app
}

func (srv *HTTPServer) Handle(hdl Handler) {
	hdl.Register(srv.app)
	srv.options.Logger.InfoContext(
		srv.ctx,
		"HTTP handler registered",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"handler", hdl.Name(),
	)
}

/* }}} */

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	Register(*fiber.App)
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

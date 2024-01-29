/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/otel/trace"
)

/* {{{ [Server] */

// HTTPServer : Server definition
type HTTPServer struct {
	config  *Config
	options *server.Options
	ctx     context.Context
	app     *fiber.App
	runing  bool
	addr    net.Addr

	sync.RWMutex
	wg sync.WaitGroup

	tracer trace.Tracer
}

var (
	servers = make(map[string]*HTTPServer, 0)
)

func Instance(name string, srv ...*HTTPServer) *HTTPServer {
	if len(srv) > 0 {
		// Set value
		servers[name] = srv[0]

		return srv[0]
	}

	return servers[name]
}

// New HTTP server (go-fiber)
func NewServer(cfg *Config, opts ...server.Option) *HTTPServer {
	ctx := context.Background()
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &HTTPServer{
		config:  cfg,
		ctx:     ctx,
		addr:    addr,
		runing:  false,
		options: server.NewOptions(),
	}

	for _, opt := range opts {
		opt(srv.options)
	}

	// Set logger
	if srv.options.Logger() == nil {
		server.Logger(logger.Logger)(srv.options)
	}

	// Set global context
	if srv.options.Context() != nil {
		srv.ctx = srv.options.Context()
	} else {
		server.Context(ctx)(srv.options)
	}

	// Set tracer
	if srv.options.TraceProvider() != nil {
		srv.tracer = srv.options.TraceProvider().Tracer(srv.Name() + "@" + srv.String())
	}

	// Register swagger
	if cfg.EnableSwagger {
		srv.Handle(NewSwagger())
	}

	app := fiber.New(
		fiber.Config{
			Prefork:               false,
			DisableStartupMessage: true,
			ServerHeader:          cfg.Name,
			AppName:               cfg.Name,
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
		tracer.NewFiberMiddleware(
			&tracer.FiberMiddlewareConfig{
				Tracer: srv.tracer,
			},
		),
		logger.NewFiberMiddleware(),
		NewMetadataMiddleware(),
	)

	srv.app = app
	server.Instance(srv.Name(), srv)
	Instance(srv.Name(), srv)
	srv.options.Logger().InfoContext(srv.ctx, "HTTP server created", "id", srv.ID(), "name", srv.Name(), "addr", addr.String())

	return srv
}

func (srv *HTTPServer) Options() *server.Options {
	return srv.options
}

func (srv *HTTPServer) Start() error {
	var (
		listener net.Listener
		err      error
	)

	srv.Lock()
	defer srv.Unlock()

	if srv.runing {
		// Runing
		return nil
	}

	// if srv.options.Handlers() != nil {
	// 	for _, hdl := range srv.options.Handlers() {
	// 		if hdl.Type() == srv.String() {
	// 			srv.options.Logger().DebugContext(srv.ctx, "Register HTTP handler", "server", srv.Name(), "name", hdl.Name())
	// 			hdl.Register(srv.Name())
	// 		}
	// 	}
	// }

	if srv.options.TLS() != nil {
		listener, err = tls.Listen(
			srv.addr.Network(),
			srv.addr.String(),
			srv.options.TLS(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "HTTP server with TLS listen failed", "error", err.Error())

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.addr.Network(),
			srv.addr.String(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "HTTP server listen failed", "error", err.Error())

			return err
		}
	}

	srv.addr = listener.Addr()
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Listener(listener)
		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "HTTP server listen failed", "error", err.Error())

			return err
		}

		srv.options.Logger().InfoContext(
			srv.ctx,
			"HTTP server closed",
			"id", srv.options.ID(),
			"server", srv.config.Name,
		)
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger().InfoContext(
		srv.ctx,
		"HTTP server listened",
		"id", srv.options.ID(),
		"server", srv.config.Name,
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

func (srv *HTTPServer) String() string {
	return "http"
}

func (srv *HTTPServer) Name() string {
	return srv.config.Name
}

func (srv *HTTPServer) ID() string {
	return srv.options.ID()
}

func (srv *HTTPServer) App() *fiber.App {
	return srv.app
}

func (srv *HTTPServer) Handle(ss any) {
	hdl, ok := ss.(HTTPHandler)
	if ok {
		hdl.Register(srv.app)
		srv.options.Logger().InfoContext(
			srv.ctx,
			"HTTP handler registered",
			"handler", hdl.Name(),
		)
	} else {
		srv.options.Logger().WarnContext(
			srv.ctx,
			"Invalid HTTP handler",
		)
	}
}

/* }}} */

/* {{{ [Handler] */
type HTTPHandler interface {
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

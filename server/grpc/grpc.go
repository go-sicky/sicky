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
 * @file grpc.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package grpc

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// GRPCServer : Server definition
type GRPCServer struct {
	config  *Config
	options *server.Options
	ctx     context.Context
	app     *grpc.Server
	runing  bool
	addr    net.Addr

	sync.RWMutex
	wg sync.WaitGroup

	tracer trace.Tracer
}

var (
	servers = make(map[string]*GRPCServer, 0)
)

func Instance(name string, clt ...*GRPCServer) *GRPCServer {
	if len(clt) > 0 {
		// Set value
		servers[name] = clt[0]

		return clt[0]
	}

	return servers[name]
}

// New GRPC server
func NewServer(cfg *Config, opts ...server.Option) *GRPCServer {
	ctx := context.Background()
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &GRPCServer{
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

	gopts := make([]grpc.ServerOption, 0)
	if srv.options.TLS() != nil {
		gopts = append(gopts, grpc.Creds(credentials.NewTLS(srv.options.TLS())))
	}

	if cfg.MaxConcurrentStreams > 0 {
		gopts = append(gopts, grpc.MaxConcurrentStreams(cfg.MaxConcurrentStreams))
	}

	if cfg.MaxHeaderListSize > 0 {
		gopts = append(gopts, grpc.MaxHeaderListSize(cfg.MaxHeaderListSize))
	}

	if cfg.MaxRecvMsgSize != 0 {
		gopts = append(gopts, grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize))
	}

	if cfg.MaxSendMsgSize != 0 {
		gopts = append(gopts, grpc.MaxSendMsgSize(cfg.MaxSendMsgSize))
	}

	if cfg.ReadBufferSize != 0 {
		gopts = append(gopts, grpc.ReadBufferSize(cfg.ReadBufferSize))
	}

	if cfg.WriteBufferSize != 0 {
		gopts = append(gopts, grpc.WriteBufferSize(cfg.WriteBufferSize))
	}

	gopts = append(gopts, grpc.ChainUnaryInterceptor(
		tracer.NewGRPCServerInterceptor(srv.tracer),
		logger.NewGRPCServerInterceptor(srv.options.Logger()),
	))
	app := grpc.NewServer(gopts...)
	reflection.Register(app)

	srv.app = app
	server.Instance(srv.Name(), srv)
	Instance(srv.Name(), srv)
	srv.options.Logger().InfoContext(srv.ctx, "GRPC server created", "id", srv.ID(), "name", srv.Name(), "addr", addr.String())

	return srv
}

func (srv *GRPCServer) Options() *server.Options {
	return srv.options
}

func (srv *GRPCServer) Start() error {
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
	// 	tt := reflect.TypeOf((*server.HandlerGRPC)(nil)).Elem()
	// 	for _, hdl := range srv.options.Handlers() {
	// 		ht := reflect.TypeOf(hdl.Hdl)
	// 		if ht.Implements(tt) {
	// 			tg, ok := hdl.Hdl.(server.HandlerGRPC)
	// 			if ok {
	// 				srv.options.Logger().DebugContext(srv.ctx, "Register GRPC handler", "server", srv.Name(), "name", tg.Name())
	// 				hdl.Type = srv.String()
	// 				tg.Register(srv)
	// 			}
	// 		}
	// 	}
	// }
	if srv.options.Handlers() != nil {
		for _, hdl := range srv.options.Handlers() {
			srv.options.Logger().DebugContext(srv.ctx, "Register GRPC handler", "server", srv.Name(), "name", hdl.Name())
			hdl.Register(srv.Name())
		}
	}

	if srv.options.TLS() != nil {
		listener, err = tls.Listen(
			srv.addr.Network(),
			srv.addr.String(),
			srv.options.TLS(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "GRPC server with TLS listen failed", "error", err.Error())

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.addr.Network(),
			srv.addr.String(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "GRPC server listen failed", "error", err.Error())

			return err
		}
	}

	srv.addr = listener.Addr()
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Serve(listener)
		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "GRPC server listen failed", "error", err.Error())

			return err
		}

		srv.options.Logger().InfoContext(
			srv.ctx,
			"GRPC server closed",
			"id", srv.options.ID(),
			"server", srv.config.Name,
		)
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger().InfoContext(
		srv.ctx,
		"GRPC server listened",
		"id", srv.options.ID(),
		"server", srv.config.Name,
		"addr", srv.addr.String())
	srv.runing = true

	return nil
}

func (srv *GRPCServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.runing {
		// Not runing
		return nil
	}

	srv.app.GracefulStop()
	srv.wg.Wait()
	srv.runing = false

	return nil
}

func (srv *GRPCServer) String() string {
	return "grpc"
}

func (srv *GRPCServer) Name() string {
	return srv.config.Name
}

func (srv *GRPCServer) ID() string {
	return srv.options.ID()
}

func (srv *GRPCServer) App() *grpc.Server {
	return srv.app
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

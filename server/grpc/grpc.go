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
	"reflect"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCServer : Server definition
type GRPCServer struct {
	ctx      context.Context
	app      *grpc.Server
	runing   bool
	logger   logger.GeneralLogger
	options  *server.Options
	handlers []*server.Handler

	sync.RWMutex
	wg sync.WaitGroup
}

// New GRPC server
func NewServer(cfg *Config, opts ...server.Option) server.Server {
	ctx := context.Background()
	baseLogger := logger.Logger
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &GRPCServer{
		ctx:    ctx,
		runing: false,
		logger: baseLogger,
		options: &server.Options{
			Name: cfg.Name,
			Addr: addr,
		},
	}

	for _, opt := range opts {
		opt(srv.options)
	}

	// Set logger
	if srv.options.Logger != nil {
		srv.logger = srv.options.Logger
	} else {
		srv.options.Logger = baseLogger
	}

	// Set global context
	if srv.options.Context != nil {
		srv.ctx = srv.options.Context
	} else {
		srv.options.Context = ctx
	}

	// Set ID
	srv.options.ID = uuid.New().String()

	gopts := make([]grpc.ServerOption, 0)
	if srv.options.TLS != nil {
		gopts = append(gopts, grpc.Creds(credentials.NewTLS(srv.options.TLS)))
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

	gopts = append(gopts, grpc.ChainUnaryInterceptor(logger.GRPCServerMiddlware))
	app := grpc.NewServer(gopts...)

	srv.app = app

	return srv
}

func (srv *GRPCServer) Options() *server.Options {
	return srv.options
}

func (srv *GRPCServer) Handle(hdl *server.Handler) error {
	srv.handlers = append(srv.handlers, hdl)

	return nil
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

	if srv.handlers != nil {
		tt := reflect.TypeOf((*server.HandlerGRPC)(nil)).Elem()
		for _, hdl := range srv.handlers {
			ht := reflect.TypeOf(hdl.Hdl)
			if ht.Implements(tt) {
				tg, ok := hdl.Hdl.(server.HandlerGRPC)
				if ok {
					tg.Register(srv.app)
				}
			}
		}
	}

	if srv.options.TLS != nil {
		listener, err = tls.Listen(
			srv.options.Addr.Network(),
			srv.options.Addr.String(),
			srv.options.TLS,
		)

		if err != nil {
			srv.logger.Error("GRPC server with TLS listen failed", "error", err.Error())

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.options.Addr.Network(),
			srv.options.Addr.String(),
		)

		if err != nil {
			srv.logger.Error("GRPC server listen failed", "error", err.Error())

			return err
		}
	}

	srv.options.Addr = listener.Addr()
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Serve(listener)
		if err != nil {
			srv.logger.Error("GRPC server listen failed", "error", err.Error())

			return err
		}

		srv.logger.Info("GRPC server closed", "server", srv.options.Name)
		srv.wg.Done()

		return nil
	}()

	srv.logger.Info("GRPC server listened", "server", srv.options.Name, "addr", srv.options.Addr.String())
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
	return srv.options.Name
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

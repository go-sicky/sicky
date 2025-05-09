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

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

/* {{{ [Server] */

// GRPCServer : Server definition
type GRPCServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *grpc.Server
	running       bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata

	sync.RWMutex
	wg sync.WaitGroup
}

// New GRPC server
func New(opts *server.Options, cfg *Config) *GRPCServer {
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
				"Advertise network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)
		}
	} else {
		advertiseAddr = addr
	}

	opts.Addr = addr
	srv := &GRPCServer{
		config:        cfg,
		ctx:           context.Background(),
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
	}

	// Set tracer
	var tr trace.Tracer
	if tracer.DefaultTracer != nil {
		tr = tracer.DefaultTracer.Tracer(srv.Name())
	}

	gopts := make([]grpc.ServerOption, 0)
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

	// Tracing
	gopts = append(gopts, grpc.ChainUnaryInterceptor(
		NewTracingInterceptor(
			TracerConfig{
				Tracer: tr,
			},
		),
	))

	// Access logger
	gopts = append(gopts, grpc.ChainUnaryInterceptor(
		NewAccessLoggerInterceptor(
			LoggerConfig{
				Logger: opts.Logger,
			},
		),
	))

	app := grpc.NewServer(gopts...)
	reflection.Register(app)
	srv.app = app
	srv.options.Logger.InfoContext(
		srv.ctx,
		"GRPC server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
	)

	server.Instance(opts.ID, srv)

	return srv
}

func (srv *GRPCServer) Context() context.Context {
	return srv.ctx
}

func (srv *GRPCServer) Options() *server.Options {
	return srv.options
}

func (srv *GRPCServer) String() string {
	return "grpc"
}

func (srv *GRPCServer) ID() uuid.UUID {
	return srv.options.ID
}

func (srv *GRPCServer) Name() string {
	return srv.options.Name
}

func (srv *GRPCServer) Start() error {
	var (
		listener net.Listener
		cert     tls.Certificate
		err      error
	)

	srv.Lock()
	defer srv.Unlock()

	if srv.running {
		// running
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
	if srv.config.AdvertiseAddress == "" {
		srv.advertiseAddr = listener.Addr()
	}

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Serve(listener)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"GRPC server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"GRPC server closed",
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
		"GRPC server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.running = true

	return nil
}

func (srv *GRPCServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.running {
		// Not running
		return nil
	}

	srv.app.GracefulStop()
	srv.wg.Wait()
	srv.options.Logger.InfoContext(
		srv.ctx,
		"GRPC server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.running = false

	return nil
}

func (srv *GRPCServer) Running() bool {
	return srv.running
}

func (srv *GRPCServer) Addr() net.Addr {
	return srv.addr
}

func (srv *GRPCServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *GRPCServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *GRPCServer) AdvertiseAddr() net.Addr {
	return srv.advertiseAddr
}

func (srv *GRPCServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.advertiseAddr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *GRPCServer) AdvertisePort() int {
	return utils.AddrToPort(srv.advertiseAddr)
}

func (srv *GRPCServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *GRPCServer) App() *grpc.Server {
	return srv.app
}

func (srv *GRPCServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		hdl.Register(srv.app)
		srv.options.Logger.DebugContext(
			srv.ctx,
			"GRPC handler registered",
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
	Register(*grpc.Server)
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

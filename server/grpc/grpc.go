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
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
)

// ErrIncompleteTLSConfig is returned when only one of TLSCertPEM/TLSKeyPEM
// is set. Falling back to plaintext silently would be a security hole, so
// startup fails fast instead.
var ErrIncompleteTLSConfig = errors.New("incomplete TLS configuration: both tls_cert_pem and tls_key_pem must be set")

// ErrShutdownTimeout is returned when GracefulStop exceeds
// ShutdownTimeout and the server is force-stopped instead.
var ErrShutdownTimeout = errors.New("graceful stop timed out")

/* {{{ [Server] */

// GRPCServer : Server definition.
type GRPCServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *grpc.Server
	running       bool
	stopping      bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata

	sync.RWMutex
	wg sync.WaitGroup
}

// New GRPC server.
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

		return nil
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveTCPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"Advertise network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)

			return nil
		}
	} else {
		advertiseAddr = addr
	}

	opts.Addr = addr
	srv := &GRPCServer{
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

	gopts := make([]grpc.ServerOption, 0)
	// ConnectionTimeout reuses the legacy field as the max connection
	// age (forced recycle incl. long-lived streams); zero disables it.
	if cfg.ConnectionTimeout > 0 || cfg.MaxConnectionIdle > 0 ||
		cfg.KeepaliveTime > 0 || cfg.KeepaliveTimeout > 0 {
		gopts = append(gopts, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     cfg.MaxConnectionIdle,
			MaxConnectionAge:      cfg.ConnectionTimeout,
			MaxConnectionAgeGrace: cfg.MaxConnectionAgeGrace,
			Time:                  cfg.KeepaliveTime,
			Timeout:               cfg.KeepaliveTimeout,
		}))
	}

	if cfg.MinPingInterval > 0 {
		gopts = append(gopts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             cfg.MinPingInterval,
			PermitWithoutStream: false,
		}))
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

	// Tracing (outer) + access logger (inner). The logger reads the
	// X-B3-* headers injected by the tracing interceptor, so this order
	// matters. A single ChainUnaryInterceptor call holds the whole chain.
	// Recovery stays outermost so panics from any inner interceptor or
	// the handler never crash the Serve loop.
	gopts = append(gopts, grpc.ChainUnaryInterceptor(
		NewRecoveryInterceptor(
			LoggerConfig{
				Logger: opts.Logger,
			},
		),
		NewTracingInterceptor(
			TracerConfig{
				Tracer: tr,
			},
		),
		NewAccessLoggerInterceptor(
			LoggerConfig{
				Logger: opts.Logger,
			},
		),
	), grpc.ChainStreamInterceptor(
		NewStreamRecoveryInterceptor(
			LoggerConfig{
				Logger: opts.Logger,
			},
		),
		NewStreamTracingInterceptor(
			TracerConfig{
				Tracer: tr,
			},
		),
		NewStreamAccessLoggerInterceptor(
			LoggerConfig{
				Logger: opts.Logger,
			},
		),
	))

	app := grpc.NewServer(gopts...)
	// BREAKING: reflection is opt-in (default off). Legacy DisableReflection=false
	// no longer enables it; set EnableReflection=true to expose descriptors.
	if cfg.EnableReflection && !cfg.DisableReflection {
		reflection.Register(app)
	}

	srv.app = app
	srv.options.Logger.InfoContext(
		srv.ctx,
		"GRPC server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
	)

	server.Set(srv)

	return srv
}

// Context returns the component context.
func (srv *GRPCServer) Context() context.Context {
	return srv.ctx
}

// Options returns the runtime options.
func (srv *GRPCServer) Options() *server.Options {
	return srv.options
}

// String returns a human-readable name.
func (srv *GRPCServer) String() string {
	return "grpc"
}

// ID returns the unique instance ID.
func (srv *GRPCServer) ID() uuid.UUID {
	return srv.options.ID
}

// Name returns the component name.
func (srv *GRPCServer) Name() string {
	return srv.options.Name
}

// Start starts the component.
func (srv *GRPCServer) Start() error {
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
				NextProtos:   []string{"h2"},
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
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"GRPC serving without TLS (insecure mode)",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"network", srv.addr.Network(),
			"address", srv.addr.String(),
		)
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

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Go(func() {
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

			return
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"GRPC server closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"addr", srv.addr.String(),
		)
	})

	srv.options.Logger.InfoContext(
		srv.ctx,
		"GRPC server listened",
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
func (srv *GRPCServer) Stop() error {
	// Check-and-flag under lock, then release: holding Lock across
	// GracefulStop/Wait would starve all RLock readers for the whole drain.
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

	// GracefulStop has no deadline: bound it, then force-stop so a
	// hung stream cannot hang Stop forever.
	var errs error
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.GracefulStop()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		app.Stop()
		<-done
		err := ErrShutdownTimeout
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"GRPC graceful stop timed out, connections force-stopped",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"timeout", timeout.String(),
			"error", err.Error(),
		)
		errs = errors.Join(errs, err)
	}

	srv.wg.Wait()

	srv.Lock()
	srv.running = false
	srv.stopping = false
	srv.Unlock()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"GRPC server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.options.RunAfterStop()

	return errs
}

// Running reports whether the component is running.
func (srv *GRPCServer) Running() bool {
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

// Addr returns the address.
func (srv *GRPCServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

// IP returns the IP.
func (srv *GRPCServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// Port returns the port.
func (srv *GRPCServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

// AdvertiseAddr returns the advertise address.
func (srv *GRPCServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

// AdvertiseIP returns the advertise IP.
func (srv *GRPCServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// AdvertisePort returns the advertise port.
func (srv *GRPCServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

// Metadata returns the metadata.
func (srv *GRPCServer) Metadata() utils.Metadata {
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
func (srv *GRPCServer) App() *grpc.Server {
	return srv.app
}

// Handle registers handlers.
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

/* {{{ [Handler]. */
type Handler interface {
	Name() string
	Type() string
	Register(srv *grpc.Server)
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

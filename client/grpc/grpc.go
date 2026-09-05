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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/tracer"
)

// How often the discovery loop retries while the registry pool is not
// initialized yet, and how often it re-syncs as a fallback in case a
// pool notification is missed.
const (
	discoveryRetryInterval  = 5 * time.Second
	discoveryResyncInterval = 30 * time.Second
)

// GRPCClient : Client definition.
type GRPCClient struct {
	config    *Config
	options   *client.Options
	ctx       context.Context
	conn      *grpc.ClientConn
	connected bool

	// Closed on Disconnect to stop the service-discovery goroutine.
	done      chan struct{}
	closeOnce sync.Once
}

// New GRPC client.
func New(opts *client.Options, cfg *Config) *GRPCClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	clt := &GRPCClient{
		config:    cfg,
		ctx:       opts.Context,
		connected: false,
		options:   opts,
		done:      make(chan struct{}),
	}

	// A half-configured TLS must never silently fall back to plaintext.
	if err := cfg.Validate(); err != nil {
		clt.options.Logger.ErrorContext(
			clt.ctx,
			"GRPC client TLS configuration incomplete",
			"client", clt.String(),
			"id", clt.options.ID,
			"name", clt.options.Name,
			"error", err.Error(),
		)

		return nil
	}

	gopts := make([]grpc.DialOption, 0)
	if cfg.TLSCertPEM != "" && cfg.TLSKeyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(cfg.TLSCertPEM), []byte(cfg.TLSKeyPEM))
		if err != nil {
			clt.options.Logger.ErrorContext(
				clt.ctx,
				"GRPC client TLS certification failed",
				"client", clt.String(),
				"id", clt.options.ID,
				"name", clt.options.Name,
				"error", err.Error(),
			)

			return nil
		}

		gopts = append(gopts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		})))
	} else {
		gopts = append(gopts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if cfg.MaxHeaderListSize > 0 {
		gopts = append(gopts, grpc.WithMaxHeaderListSize(cfg.MaxHeaderListSize))
	}

	if cfg.MaxMsgSize != 0 {
		gopts = append(gopts, grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.MaxMsgSize),
			grpc.MaxCallSendMsgSize(cfg.MaxMsgSize),
		))
	}

	if cfg.ReadBufferSize != 0 {
		gopts = append(gopts, grpc.WithReadBufferSize(cfg.ReadBufferSize))
	}

	if cfg.WriteBufferSize != 0 {
		gopts = append(gopts, grpc.WithWriteBufferSize(cfg.WriteBufferSize))
	}

	// Set tracer
	var tr trace.Tracer
	if tracer.Default() != nil {
		tr = tracer.Default().Tracer(clt.Name())
	}

	// Tracing (outer) + logger (inner), single chain each. The tracing
	// entry is only appended when a tracer exists; the logger always runs.
	unaryChain := make([]grpc.UnaryClientInterceptor, 0, 2)
	streamChain := make([]grpc.StreamClientInterceptor, 0, 2)
	if tr != nil {
		unaryChain = append(unaryChain, NewClientTracingInterceptor(tr))
		streamChain = append(streamChain, NewClientStreamTracingInterceptor(tr))
	}

	unaryChain = append(unaryChain, NewClientLoggerInterceptor(clt.options.Logger))
	streamChain = append(streamChain, NewClientStreamLoggerInterceptor(clt.options.Logger))
	gopts = append(gopts,
		grpc.WithChainUnaryInterceptor(unaryChain...),
		grpc.WithChainStreamInterceptor(streamChain...),
	)

	// Resolver
	r := manual.NewBuilderWithScheme("sicky")
	r.ResolveNowCallback = sickyResolveNow
	r.UpdateStateCallback = sickyUpdateState
	r.BuildCallback = sickyBuild
	r.CloseCallback = sickyClose
	r.InitialState(resolver.State{})

	sc := &grpcServiceConfig{}

	// Timeout
	if cfg.ConnectionTimeout > 0 {
		sc.Timeout = cfg.ConnectionTimeout.String()
	}

	// Client connection
	var (
		conn *grpc.ClientConn
		err  error
	)

	if cfg.Addr != "" {
		// Override default service config
		b, _ := json.Marshal(sc)
		gopts = append(gopts, grpc.WithDefaultServiceConfig(string(b)))

		conn, err = grpc.NewClient(cfg.Addr, gopts...)
	} else {
		// Balancer
		balancer := make(map[string]map[string]any)
		balancer[cfg.Balancer] = make(map[string]any)
		sc.LoadBalancingConfig = append(sc.LoadBalancingConfig, balancer)

		// Override default service config
		b, _ := json.Marshal(sc)
		gopts = append(gopts,
			grpc.WithDefaultServiceConfig(string(b)),
			grpc.WithResolvers(r),
		)

		conn, err = grpc.NewClient("sicky:///"+cfg.Service, gopts...)
	}

	if err != nil {
		clt.options.Logger.ErrorContext(
			clt.ctx,
			"GRPC client dial failed",
			"client", clt.String(),
			"id", clt.options.ID,
			"name", clt.options.Name,
			"balancer", cfg.Balancer,
			"error", err.Error(),
		)

		return nil
	}

	conn.Connect()
	clt.conn = conn
	clt.options.Logger.InfoContext(
		clt.ctx,
		"GRPC client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
		"balancer", cfg.Balancer,
		"service", cfg.Service,
		"address", cfg.Addr,
	)

	client.Set(clt)

	// Service discovery: in Service mode (no static Addr) keep the manual
	// resolver in sync with the registry pool. Direct-Addr mode needs no
	// updates.
	if cfg.Addr == "" && cfg.Service != "" {
		r.InitialState(resolver.State{Addresses: resolveGRPCAddrs(cfg.Service)})
		go watchRegistryPool(clt, r, cfg.Service)
	}

	return clt
}

// resolveGRPCAddrs snapshots the registry pool for service and returns the
// advertised grpc endpoints as resolver addresses.
func resolveGRPCAddrs(service string) []resolver.Address {
	ins := registry.GetInstances(service)
	addrs := make([]resolver.Address, 0, len(ins))
	for _, in := range ins {
		if in == nil {
			continue
		}

		for _, srv := range in.Servers {
			if srv == nil || srv.Type != "grpc" {
				continue
			}

			addr := srv.AdvertiseAddress
			if addr == "" {
				addr = in.AdvertiseAddress
			}

			if addr == "" || srv.Port <= 0 {
				continue
			}

			addrs = append(addrs, resolver.Address{
				Addr: fmt.Sprintf("%s:%d", addr, srv.Port),
			})
		}
	}

	return addrs
}

// watchRegistryPool pushes registry pool updates into the manual resolver
// until the client is disconnected. A periodic resync guards against a
// missed notification; the loop never blocks shutdown.
func watchRegistryPool(clt *GRPCClient, r *manual.Resolver, service string) {
	resync := time.NewTicker(discoveryResyncInterval)
	defer resync.Stop()

	for {
		ch := registry.NotifyChan()
		if ch == nil {
			// Pool not initialized yet (client created before InitPool):
			// retry instead of exiting so discovery still comes up.
			select {
			case <-clt.done:
				return
			case <-time.After(discoveryRetryInterval):
				r.UpdateState(resolver.State{Addresses: resolveGRPCAddrs(service)})
			}

			continue
		}

		select {
		case <-clt.done:
			return
		case ev := <-ch:
			if ev.Changed {
				r.UpdateState(resolver.State{Addresses: resolveGRPCAddrs(service)})
			}
		case <-resync.C:
			r.UpdateState(resolver.State{Addresses: resolveGRPCAddrs(service)})
		}
	}
}

// Options returns the runtime options.
func (clt *GRPCClient) Options() *client.Options {
	return clt.options
}

// Context returns the component context.
func (clt *GRPCClient) Context() context.Context {
	return clt.ctx
}

// Connect connects to the backend.
func (clt *GRPCClient) Connect() error {
	clt.connected = true

	return nil
}

// Disconnect disconnects from the backend.
func (clt *GRPCClient) Disconnect() error {
	clt.connected = false
	clt.closeOnce.Do(func() { close(clt.done) })

	return clt.conn.Close()
}

// Call implements client.Client. It only bumps the call counter; real RPCs
// go through Invoke (unary) or NewStream (streaming).
func (clt *GRPCClient) Call() error {
	metrics.NumGRPCClientCallCounter.Inc()

	return nil
}

// String returns a human-readable name.
func (clt *GRPCClient) String() string {
	return "grpc"
}

// Name returns the component name.
func (clt *GRPCClient) Name() string {
	return clt.options.Name
}

// ID returns the unique instance ID.
func (clt *GRPCClient) ID() uuid.UUID {
	return clt.options.ID
}

// For GRPC client connection.
func (clt *GRPCClient) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	// Invoke logger
	clt.options.Logger.DebugContext(
		ctx,
		"Invoke GRPC call",
		"client", clt.options.ID,
		"name", clt.options.Name,
		"method", method,
		"args", args,
		"reply", reply,
	)
	metrics.NumGRPCClientCallCounter.Inc()
	err := clt.conn.Invoke(ctx, method, args, reply, opts...)
	if err != nil {
		clt.options.Logger.ErrorContext(
			ctx,
			"Invoke GRPC call failed",
			"client", clt.options.ID,
			"name", clt.options.Name,
			"method", method,
			"error", err.Error(),
		)

		return fmt.Errorf("grpc client invoke (method %s): %w", method, err)
	}

	return nil
}

// NewStream creates a new Stream.
func (clt *GRPCClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	// Stream call
	clt.options.Logger.DebugContext(
		ctx,
		"Stream GRPC call",
		"client", clt.options.ID,
		"name", clt.options.Name,
		"method", method,
	)
	stream, err := clt.conn.NewStream(ctx, desc, method, opts...)
	if err != nil {
		clt.options.Logger.ErrorContext(
			ctx,
			"Stream GRPC call failed",
			"client", clt.options.ID,
			"name", clt.options.Name,
			"method", method,
			"error", err.Error(),
		)

		return nil, fmt.Errorf("grpc client new stream (method %s): %w", method, err)
	}

	return stream, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

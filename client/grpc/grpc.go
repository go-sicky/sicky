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
	"encoding/json"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/registry"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

// GRPCClient : Client definition
type GRPCClient struct {
	config    *Config
	options   *client.Options
	ctx       context.Context
	conn      *grpc.ClientConn
	connected bool
}

// New GRPC client
func New(opts *client.Options, cfg *Config) *GRPCClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	clt := &GRPCClient{
		config:    cfg,
		ctx:       context.Background(),
		connected: false,
		options:   opts,
	}

	// clt.addr = addr
	gopts := make([]grpc.DialOption, 0)
	if cfg.TLSCertPEM != "" && cfg.TLSKeyPEM != "" {
		// SSL
	} else {
		gopts = append(gopts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if cfg.MaxHeaderListSize > 0 {
		gopts = append(gopts, grpc.WithMaxHeaderListSize(cfg.MaxHeaderListSize))
	}

	if cfg.MaxMsgSize != 0 {
		gopts = append(gopts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.MaxMsgSize)))
		gopts = append(gopts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(cfg.MaxMsgSize)))
	}

	if cfg.ReadBufferSize != 0 {
		gopts = append(gopts, grpc.WithReadBufferSize(cfg.ReadBufferSize))
	}

	if cfg.WriteBufferSize != 0 {
		gopts = append(gopts, grpc.WithWriteBufferSize(cfg.WriteBufferSize))
	}

	// gopts = append(gopts,
	// 	grpc.WithChainUnaryInterceptor(
	// 		tracer.NewGRPCClientInterceptor(clt.tracer),
	// 		logger.NewGRPCClientInterceptor(clt.options.Logger()),
	// 	),
	// )
	gopts = append(gopts, grpc.WithChainUnaryInterceptor(
		NewClientLoggerInterceptor(clt.options.Logger),
	))

	// Resolver
	r := manual.NewBuilderWithScheme("sicky")
	r.ResolveNowCallback = sickyResolveNow
	r.UpdateStateCallback = sickyUpdateState
	r.BuildCallback = sickyBuild
	r.CloseCallback = sickyClose
	r.InitialState(resolver.State{})
	gopts = append(gopts, grpc.WithResolvers(r))

	sc := &grpcServiceConfig{}

	// Balancer
	balancer := make(map[string]map[string]any)
	balancer[cfg.Balancer] = make(map[string]any)
	sc.LoadBalancingConfig = append(sc.LoadBalancingConfig, balancer)

	// Timeout
	if cfg.ConnectionTimeout > 0 {
		sc.Timeout = cfg.ConnectionTimeout.String()
	}

	// Override default service config
	b, _ := json.Marshal(sc)
	gopts = append(gopts, grpc.WithDefaultServiceConfig(string(b)))

	// Client connection
	var (
		conn *grpc.ClientConn
		err  error
	)

	if cfg.Addr != "" {
		conn, err = grpc.NewClient(cfg.Addr, gopts...)
	} else {
		conn, err = grpc.NewClient("sicky:///"+cfg.Service, gopts...)
	}

	if err != nil {
		clt.options.Logger.ErrorContext(
			clt.ctx,
			"GRPC dial failed",
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
		"Client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
		"balancer", cfg.Balancer,
		"service", cfg.Service,
		"address", cfg.Addr,
	)

	client.Instance(opts.ID, clt)

	// Pool notifier
	go func() {
		for ev := range registry.PoolChan {
			if cfg.Service != "" && ev.Changed {
				ins := registry.GetInstances(cfg.Service)
				if len(ins) != 0 && r.CC != nil {
					addrs := make([]resolver.Address, 0)
					for _, in := range ins {
						addr := resolver.Address{
							Addr: in.Addr.String(),
						}

						addrs = append(addrs, addr)
						clt.options.Logger.TraceContext(
							clt.ctx,
							"Append address to GRPC client resolver state",
							"id", clt.options.ID,
							"service", cfg.Service,
							"address", in.Addr.String(),
						)
					}

					r.UpdateState(resolver.State{Addresses: addrs})
				}
			}
		}
	}()

	return clt
}

func (clt *GRPCClient) Options() *client.Options {
	return clt.options
}

func (clt *GRPCClient) Context() context.Context {
	return clt.ctx
}

func (clt *GRPCClient) Connect() error {
	clt.connected = true

	return nil
}

func (clt *GRPCClient) Disconnect() error {
	clt.connected = false

	return clt.conn.Close()
}

func (clt *GRPCClient) Call() error {
	return nil
}

func (clt *GRPCClient) String() string {
	return "grpc"
}

func (clt *GRPCClient) Name() string {
	return clt.options.Name
}

func (clt *GRPCClient) ID() uuid.UUID {
	return clt.options.ID
}

// For GRPC client connection
func (clt *GRPCClient) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
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
	}

	return err
}

func (clt GRPCClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
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

		return nil, err
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

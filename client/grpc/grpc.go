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
	"net"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClient : Client definition
type GRPCClient struct {
	config  *Config
	options *client.Options
	ctx     context.Context
	conn    *grpc.ClientConn
	addr    net.Addr
}

// New GRPC client
func NewClient(cfg *Config, opts ...client.Option) client.Client {
	ctx := context.Background()
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	clt := &GRPCClient{
		config:  cfg,
		ctx:     ctx,
		addr:    addr,
		options: client.NewOptions(),
	}

	for _, opt := range opts {
		opt(clt.options)
	}

	// Set logger
	if clt.options.Logger() == nil {
		client.Logger(logger.Logger)(clt.options)
	}

	// Set global context
	if clt.options.Context() != nil {
		clt.ctx = clt.options.Context()
	} else {
		client.Context(ctx)(clt.options)
	}

	gopts := make([]grpc.DialOption, 0)
	if clt.options.TLS() != nil {
		gopts = append(gopts, grpc.WithTransportCredentials(credentials.NewTLS(clt.options.TLS())))
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

	gopts = append(gopts, grpc.WithChainUnaryInterceptor(logger.GRPCClientMiddleware))
	conn, err := grpc.Dial(addr.String(), gopts...)
	if err != nil {
		clt.options.Logger().ErrorContext(clt.ctx, "GRPC dial failed", "error", err.Error())

		return nil
	}

	clt.conn = conn

	return clt
}

func (clt *GRPCClient) Options() *client.Options {
	return clt.options
}

func (clt *GRPCClient) Call() error {
	return nil
}

func (clt *GRPCClient) String() string {
	return "grpc"
}

func (clt *GRPCClient) Name() string {
	return clt.config.Name
}

func (clt *GRPCClient) ID() string {
	return clt.options.ID()
}

func (clt *GRPCClient) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return clt.conn.Invoke(ctx, method, args, reply, opts...)
}

func (clt GRPCClient) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return clt.conn.NewStream(ctx, desc, method, opts...)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

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
 * @file nats.go
 * @package nats
 * @author Dr.NP <np@herewe.tech>
 * @since 12/08/2023
 */

package nats

import (
	"context"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/logger"
	"go.opentelemetry.io/otel/trace"
)

// NatsClient : Client definition
type NatsClient struct {
	config  *Config
	options *client.Options
	ctx     context.Context

	tracer trace.Tracer
}

var (
	clients = make(map[string]*NatsClient, 0)
)

func Instance(name string, clt ...*NatsClient) *NatsClient {
	if len(clt) > 0 {
		// Set value
		clients[name] = clt[0]

		return clt[0]
	}

	return clients[name]
}

// New HTTP client
func NewClient(cfg *Config, opts ...client.Option) *NatsClient {
	ctx := context.Background()
	clt := &NatsClient{
		config:  cfg,
		ctx:     ctx,
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

	// Set tracer
	if clt.options.TraceProvider() != nil {
		clt.tracer = clt.options.TraceProvider().Tracer(clt.Name() + "@" + clt.String())
	}

	client.Instance(clt.Name(), clt)
	Instance(clt.Name(), clt)
	clt.options.Logger().InfoContext(clt.ctx, "Nats client created", "id", clt.ID(), "name", clt.Name())

	return clt
}

func (clt *NatsClient) Options() *client.Options {
	return clt.options
}

func (clt *NatsClient) Call() error {
	return nil
}

func (clt *NatsClient) String() string {
	return "nats"
}

func (clt *NatsClient) Name() string {
	return clt.config.Name
}

func (clt *NatsClient) ID() string {
	return clt.options.ID()
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

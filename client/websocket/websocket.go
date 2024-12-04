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
 * @file websocket.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 01/29/2024
 */

package websocket

import (
	"context"

	"github.com/go-sicky/sicky/client"
)

type WebsocketClient struct {
	config  *Config
	options *client.Options
	ctx     context.Context

	//tracer trace.Tracer
}

// var (
// 	clients = make(map[string]*WebsocketClient, 0)
// )

// func Instance(name string, clt ...*WebsocketClient) *WebsocketClient {
// 	if len(clt) > 0 {
// 		// Set value
// 		clients[name] = clt[0]

// 		return clt[0]
// 	}

// 	return clients[name]
// }

// New websocket client
func New(opts *client.Options, cfg *Config) *WebsocketClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	clt := &WebsocketClient{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	// for _, opt := range opts {
	// 	opt(clt.options)
	// }

	// // Set logger
	// if clt.options.Logger() == nil {
	// 	client.Logger(logger.Logger)(clt.options)
	// }

	// // Set global context
	// if clt.options.Context() != nil {
	// 	clt.ctx = clt.options.Context()
	// } else {
	// 	client.Context(ctx)(clt.options)
	// }

	// // Set tracer
	// if clt.options.TraceProvider() != nil {
	// 	clt.tracer = clt.options.TraceProvider().Tracer(clt.Name() + "@" + clt.String())
	// }

	// client.Instance(clt.Name(), clt)
	// Instance(clt.Name(), clt)
	// clt.options.Logger().InfoContext(clt.ctx, "Websocket client created", "id", clt.ID(), "name", clt.Name())
	clt.options.Logger.InfoContext(
		clt.ctx,
		"Client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
	)

	client.Instance(opts.ID, clt)

	return clt
}

func (clt *WebsocketClient) Options() *client.Options {
	return clt.options
}

func (clt *WebsocketClient) Connect() error {
	return nil
}

func (clt *WebsocketClient) Disconnect() error {
	return nil
}

func (clt *WebsocketClient) Call() error {
	return nil
}

func (clt *WebsocketClient) String() string {
	return "websocket"
}

func (clt *WebsocketClient) Name() string {
	return clt.options.Name
}

func (clt *WebsocketClient) ID() string {
	return clt.options.ID.String()
}

func (clt *WebsocketClient) Handle(hdl WebsocketHandler) {

}

/* {{{ [Handler] */
type WebsocketHandler interface {
	Name() string
	Type() string
	OnConnect(string) error
	OnClose(string) error
	OnError(string, error) error
	OnData(string, []byte) error
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

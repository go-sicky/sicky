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
 * @file tcp.go
 * @package tcp
 * @author Dr.NP <np@herewe.tech>
 * @since 03/02/2025
 */

package tcp

import (
	"context"
	"net"

	"github.com/go-sicky/sicky/client"
	"github.com/google/uuid"
)

type TCPClient struct {
	config    *Config
	options   *client.Options
	ctx       context.Context
	conn      net.TCPConn
	connected bool
	addr      *net.TCPAddr
}

func New(opts *client.Options, cfg *Config) *TCPClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	addr, _ := net.ResolveTCPAddr("tcp", cfg.Addr)
	clt := &TCPClient{
		config:    cfg,
		options:   opts,
		ctx:       opts.Context,
		addr:      addr,
		connected: false,
	}

	clt.options.Logger.InfoContext(
		clt.ctx,
		"TCP client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
		"addr", cfg.Addr,
	)

	client.Set(clt)

	return clt
}

func (clt *TCPClient) Options() *client.Options {
	return clt.options
}

func (clt *TCPClient) Context() context.Context {
	return clt.ctx
}

func (clt *TCPClient) String() string {
	return "tcp"
}

func (clt *TCPClient) ID() uuid.UUID {
	return clt.options.ID
}

func (clt *TCPClient) Name() string {
	return clt.options.Name
}

func (clt *TCPClient) Connect() error {
	if clt.connected {
		return nil
	}

	conn, err := net.DialTCP("tcp", nil, clt.addr)
	if err != nil {
		return err
	}

	clt.conn = *conn
	clt.connected = true

	return nil
}

func (clt *TCPClient) Disconnect() error {
	if !clt.connected {
		return nil
	}

	clt.conn.Close()
	clt.connected = false

	return nil
}

func (clt *TCPClient) Call() error {
	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

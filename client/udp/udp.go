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
 * @file udp.go
 * @package udp
 * @author Dr.NP <np@herewe.tech>
 * @since 01/16/2025
 */

package udp

import (
	"context"
	"net"

	"github.com/go-sicky/sicky/client"
	"github.com/google/uuid"
)

type UDPClient struct {
	config    *Config
	options   *client.Options
	ctx       context.Context
	conn      *net.UDPConn
	connected bool
	addr      *net.UDPAddr
}

func New(opts *client.Options, cfg *Config) *UDPClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	addr, _ := net.ResolveUDPAddr("udp", cfg.Addr)
	clt := &UDPClient{
		config:    cfg,
		ctx:       opts.Context,
		addr:      addr,
		connected: false,
		options:   opts,
	}

	clt.options.Logger.InfoContext(
		clt.ctx,
		"UDP client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
		"addr", cfg.Addr,
	)

	client.Set(clt)

	return clt
}

func (clt *UDPClient) Options() *client.Options {
	return clt.options
}

func (clt *UDPClient) Context() context.Context {
	return clt.ctx
}

func (clt *UDPClient) String() string {
	return "udp"
}

func (clt *UDPClient) ID() uuid.UUID {
	return clt.options.ID
}

func (clt *UDPClient) Name() string {
	return clt.options.Name
}

func (clt *UDPClient) Connect() error {
	if clt.connected {
		return nil
	}

	conn, err := net.DialUDP("udp", nil, clt.addr)
	if err != nil {
		return err
	}

	clt.conn = conn
	clt.connected = true

	return nil
}

func (clt *UDPClient) Disconnect() error {
	if !clt.connected {
		return nil
	}

	err := clt.conn.Close()
	if err != nil {
		return err
	}

	clt.connected = false

	return nil
}

func (clt *UDPClient) Call() error {
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

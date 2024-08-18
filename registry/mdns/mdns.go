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
 * @file mdns.go
 * @package mdns
 * @author Dr.NP <np@herewe.tech>
 * @since 08/10/2024
 */

package mdns

import (
	"context"

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
)

type MDNS struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
}

func New(opts *registry.Options, cfg *Config) *MDNS {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &MDNS{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Registry created",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	return rg
}

func (rg *MDNS) Context() context.Context {
	return rg.ctx
}

func (rg *MDNS) Options() *registry.Options {
	return rg.options
}

func (rg *MDNS) String() string {
	return "mdns"
}

func (rg *MDNS) Register(srv server.Server) error {
	rg.options.Logger.InfoContext(
		rg.ctx,
		"Server registered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"server", srv.String(),
		"server_id", srv.Options().ID,
		"server_name", srv.Options().Name,
	)

	return nil
}

func (rg *MDNS) Deregister(srv server.Server) error {
	rg.options.Logger.InfoContext(
		rg.ctx,
		"Server deregistered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"server", srv.String(),
		"server_id", srv.Options().ID,
		"server_name", srv.Options().Name,
	)

	return nil
}

func (rg *MDNS) Watch() error {
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

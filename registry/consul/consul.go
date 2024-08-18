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
 * @file consul.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package consul

import (
	"context"

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
)

type Consul struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
}

func New(opts *registry.Options, cfg *Config) *Consul {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &Consul{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	return rg
}

func (rg *Consul) Context() context.Context {
	return rg.ctx
}

func (rg *Consul) Options() *registry.Options {
	return rg.options
}

func (rg *Consul) String() string {
	return "mdns"
}

func (rg *Consul) Register(server.Server) error {
	return nil
}

func (rg *Consul) Deregister(server.Server) error {
	return nil
}

func (rg *Consul) Watch() error {
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

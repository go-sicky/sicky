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
 * @file local.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 02/22/2026
 */

package local

import (
	"context"

	"github.com/go-sicky/sicky/registry"
	"github.com/google/uuid"
)

type Local struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
}

func New(opts *registry.Options, cfg *Config) *Local {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &Local{
		config:  cfg,
		options: opts,
	}

	registry.Set(rg)

	return rg
}

func (rg *Local) Context() context.Context {
	return rg.options.Context
}

func (rg *Local) Options() *registry.Options {
	return rg.options
}

func (rg *Local) String() string {
	return "local"
}

func (rg *Local) ID() uuid.UUID {
	return rg.options.ID
}

func (rg *Local) Name() string {
	return rg.options.Name
}

func (rg *Local) Register(ins *registry.Instance) error {
	return nil
}

func (rg *Local) Deregister(id uuid.UUID) error {
	return nil
}

func (rg *Local) CheckInstance(id uuid.UUID) bool {
	return true
}

func (rg *Local) Load() ([]*registry.Instance, error) {
	return nil, nil
}

func (rg *Local) Watch() error {
	return nil
}

func (rg *Local) Purge() error {
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

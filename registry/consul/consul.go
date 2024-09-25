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
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type Consul struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
	client  *api.Client
	watcher *Watcher
}

func New(opts *registry.Options, cfg *Config) *Consul {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &Consul{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	apiCfg := api.DefaultConfig()
	apiCfg.Address = cfg.Addr
	client, err := api.NewClient(apiCfg)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Registry connection failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		return nil
	}

	rg.client = client
	w, err := newWatcher(rg)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Create watcher failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)
	} else {
		rg.watcher = w
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Registry created",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	registry.Instance(opts.ID, rg)

	return rg
}

func (rg *Consul) Context() context.Context {
	return rg.ctx
}

func (rg *Consul) Options() *registry.Options {
	return rg.options
}

func (rg *Consul) String() string {
	return "consul"
}

func (rg *Consul) ID() uuid.UUID {
	return rg.options.ID
}

func (rg *Consul) Name() string {
	return rg.options.Name
}

func (rg *Consul) Register(srv server.Server) error {
	reg := &api.AgentServiceRegistration{
		ID:      srv.Options().ID.String(),
		Name:    srv.Options().Name,
		Address: srv.IP().String(),
		Port:    srv.Port(),
		Meta:    srv.Metadata(),
	}
	err := rg.client.Agent().ServiceRegister(reg)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Server register failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"server", srv.String(),
			"server_id", srv.Options().ID.String(),
			"server_name", srv.Options().Name,
			"server_addr", srv.IP().String(),
			"server_port", srv.Port(),
			"error", err.Error(),
		)

		return err
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Server registered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"server", srv.String(),
		"server_id", srv.Options().ID.String(),
		"server_name", srv.Options().Name,
		"server_addr", srv.IP().String(),
		"server_port", srv.Port(),
	)

	return nil
}

func (rg *Consul) Deregister(srv server.Server) error {
	err := rg.client.Agent().ServiceDeregister(srv.Options().ID.String())
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Server deregister failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"server", srv.String(),
			"server_id", srv.Options().ID.String(),
			"server_name", srv.Options().Name,
			"error", err.Error(),
		)

		return err
	}

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

func (rg *Consul) Watch() error {
	if rg.watcher != nil {
		rg.watcher.Start()

		rg.options.Logger.InfoContext(
			rg.ctx,
			"Registry watcher start",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)
	} else {
		rg.options.Logger.WarnContext(
			rg.ctx,
			"Registry has no watcher",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)
	}

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

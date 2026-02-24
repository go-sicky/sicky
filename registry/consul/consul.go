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
 * @file consul.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package consul

import (
	"context"

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
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
		ctx:     opts.Context,
		options: opts,
	}

	apiCfg := api.DefaultConfig()
	apiCfg.Address = cfg.Endpoint
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

	registry.Set(rg)

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

func (rg *Consul) Register(ins *registry.Instance) error {
	// reg := &api.AgentServiceRegistration{
	// 	ID:      srv.Options().ID.String(),
	// 	Name:    srv.Options().Name,
	// 	Address: srv.AdvertiseIP().String(),
	// 	Port:    srv.AdvertisePort(),
	// 	Meta:    srv.Metadata(),
	// }
	// err := rg.client.Agent().ServiceRegister(reg)
	// if err != nil {
	// 	rg.options.Logger.ErrorContext(
	// 		rg.ctx,
	// 		"Server register failed",
	// 		"registry", rg.String(),
	// 		"id", rg.options.ID,
	// 		"name", rg.options.Name,
	// 		"server", srv.String(),
	// 		"server_id", srv.Options().ID.String(),
	// 		"server_name", srv.Options().Name,
	// 		"server_addr", srv.IP().String(),
	// 		"server_port", srv.Port(),
	// 		"error", err.Error(),
	// 	)

	// 	return err
	// }
	reg := &api.AgentServiceRegistration{
		Kind:    api.ServiceKindTypical,
		ID:      ins.ID.String(),
		Name:    ins.ServiceMame,
		Address: ins.ManagerAddress,
		Port:    ins.ManagerPort,
		Meta: map[string]string{
			"servers": utils.JSONAnyString(ins.Servers),
			"topics":  utils.JSONAnyString(ins.Topics),
		},
	}
	err := rg.client.Agent().ServiceRegister(reg)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Service register failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"manager_address", ins.ManagerAddress,
			"manager_port", ins.ManagerPort,
			"service_name", ins.ServiceMame,
			"service_id", ins.ID.String(),
			"error", err.Error(),
		)

		return err
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Service registered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"manager_address", ins.ManagerAddress,
		"manager_port", ins.ManagerPort,
		"service_name", ins.ServiceMame,
		"service_id", ins.ID.String(),
	)

	return nil
}

func (rg *Consul) Deregister(id uuid.UUID) error {
	// err := rg.client.Agent().ServiceDeregister(srv.Options().ID.String())
	// if err != nil {
	// 	rg.options.Logger.ErrorContext(
	// 		rg.ctx,
	// 		"Server deregister failed",
	// 		"registry", rg.String(),
	// 		"id", rg.options.ID,
	// 		"name", rg.options.Name,
	// 		"server", srv.String(),
	// 		"server_id", srv.Options().ID.String(),
	// 		"server_name", srv.Options().Name,
	// 		"error", err.Error(),
	// 	)

	// 	return err
	// }
	err := rg.client.Agent().ServiceDeregister(id.String())
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Service deregister failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"service_id", id.String(),
			"error", err.Error(),
		)

		return err
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Service deregistered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"service_id", id.String(),
	)

	return nil
}

func (rg *Consul) CheckInstance(id uuid.UUID) bool {
	svcs, err := rg.client.Agent().Services()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Get consul services failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		return false
	}

	_, ok := svcs[id.String()]

	return ok
}

func (rg *Consul) Watch() error {
	if rg.watcher != nil {
		rg.watcher.Start()

		rg.options.Logger.InfoContext(
			rg.ctx,
			"Consul registry watcher start",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)
	} else {
		rg.options.Logger.WarnContext(
			rg.ctx,
			"Consul registry has no watcher",
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

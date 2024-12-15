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
 * @file mdns.go
 * @package mdns
 * @author Dr.NP <np@herewe.tech>
 * @since 08/10/2024
 */

package mdns

import (
	"context"
	"net"
	"strings"

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
	"github.com/grandcat/zeroconf"
)

const (
	serviceWildcard = "_services._dns-sd._udp"
)

var (
	MdnsRegisters = make(map[uuid.UUID]*zeroconf.Server)
)

type MDNS struct {
	config   *Config
	ctx      context.Context
	options  *registry.Options
	resolver *zeroconf.Resolver
	watcched map[string]bool
}

func New(opts *registry.Options, cfg *Config) *MDNS {
	opts = opts.Ensure()
	cfg = cfg.Ensure()
	rs, err := zeroconf.NewResolver(nil)
	if err != nil {
		opts.Logger.Fatal(
			"zeroconf network resolver init failed",
			"error", err.Error(),
		)
	}

	rg := &MDNS{
		config:   cfg,
		ctx:      context.Background(),
		options:  opts,
		resolver: rs,
		watcched: make(map[string]bool),
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

func (rg *MDNS) Context() context.Context {
	return rg.ctx
}

func (rg *MDNS) Options() *registry.Options {
	return rg.options
}

func (rg *MDNS) String() string {
	return "mdns"
}

func (rg *MDNS) ID() uuid.UUID {
	return rg.options.ID
}

func (rg *MDNS) Name() string {
	return rg.options.Name
}

func (rg *MDNS) Register(srv server.Server) error {
	service := "_sicky._" + strings.ToLower(srv.String()) + "._" + strings.ToLower(srv.Addr().Network())
	zsrv, err := zeroconf.Register(
		srv.Options().ID.String(),
		service,
		rg.config.Domain,
		srv.Port(),
		srv.Metadata().Strings(),
		nil,
	)
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

	MdnsRegisters[srv.Options().ID] = zsrv
	rg.options.Logger.InfoContext(
		rg.ctx,
		"Server registered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"server", srv.String(),
		"server_id", srv.Options().ID,
		"server_name", srv.Options().Name,
		"service", service,
		"domain", rg.config.Domain,
	)

	return nil
}

func (rg *MDNS) Deregister(srv server.Server) error {
	zsrv, ok := MdnsRegisters[srv.Options().ID]
	if ok && zsrv != nil {
		zsrv.Shutdown()
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
	}

	return nil
}

func (rg *MDNS) CheckInstance(id string) bool {
	return false
}

func (rg *MDNS) Watch() error {
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			parts := strings.Split(strings.ToLower(entry.Instance), ".")
			if len(parts) == 4 && parts[0] == "_sicky" {
				// Add watch
				if parts[1] == "_grpc" ||
					parts[1] == "_http" ||
					parts[1] == "_websocket" ||
					parts[1] == "_udp" {
					if !rg.watcched[entry.Instance] {
						rg.watcched[entry.Instance] = true
						go func() {
							rg.options.Logger.DebugContext(
								rg.ctx,
								"zeroconf watch",
								"registry", rg.String(),
								"id", rg.options.ID,
								"name", rg.options.Name,
								"service", entry.Instance,
							)
							rg.resolver.Browse(rg.ctx, parts[0]+"."+parts[1]+"."+parts[2], rg.config.Domain, entries)
						}()
					}
				}
			} else {
				// Instance
				meta := utils.MetadataFromStrings(entry.Text)
				ins := &registry.Ins{
					ID:       entry.Instance,
					Metadata: meta,
				}
				ins.Service = meta.Value("name", "")
				network := strings.ToLower(meta.Value("network", "tcp"))
				address := strings.ToLower(meta.Value("address", ":0"))
				switch network {
				case "tcp", "tcp4", "tcp6":
					ins.Addr, _ = net.ResolveTCPAddr(network, address)
				case "udp", "udp4", "udp6":
					ins.Addr, _ = net.ResolveUDPAddr(network, address)
				case "unix", "unixpacket":
					ins.Addr, _ = net.ResolveUnixAddr(network, address)
				}
				registry.RegisterInstance(ins)
				rg.options.Logger.DebugContext(
					rg.ctx,
					"registry watch event",
					"registry", rg.String(),
					"id", rg.options.ID,
					"name", rg.options.Name,
					"service", ins.Service,
				)
			}
		}
	}(entries)

	// Check services
	go func() {
		rg.resolver.Browse(rg.ctx, serviceWildcard, "", entries)
	}()

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Registry watched",
		"registry", rg.String(),
		"id", rg.options.ID.String(),
		"name", rg.options.Name,
	)

	return nil
}

/*
 * 	 variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

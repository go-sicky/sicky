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
 * @file watcher.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 09/19/2024
 */

package consul

import (
	"net"
	"strings"
	"sync"

	"github.com/go-sicky/sicky/registry"
	"github.com/hashicorp/consul/api/watch"
)

type Watcher struct {
	addr      string
	watchPlan *watch.Plan

	sync.RWMutex
}

func newWatcher(rg *Consul) (*Watcher, error) {
	w := &Watcher{
		addr: rg.config.Addr,
	}
	params := map[string]any{
		"type": "services",
	}

	wp, err := watch.Parse(params)
	if err != nil {
		return nil, err
	}

	// Update signal
	wp.HybridHandler = func(p watch.BlockingParamVal, data any) {
		switch data.(type) {
		case map[string][]string:
			list, err := rg.client.Agent().Services()
			if err != nil {
				rg.options.Logger.ErrorContext(
					rg.ctx,
					"Grab services list failed",
					"registry", rg.String(),
					"id", rg.options.ID,
					"name", rg.options.Name,
					"error", err.Error(),
				)

				return
			}

			for n, v := range list {
				if n != "consul" {
					// Sicky service
					ins := &registry.Ins{
						ID:       v.ID,
						Service:  v.Service,
						Metadata: v.Meta,
					}
					network := strings.ToLower(ins.Metadata.Value("network", "tcp"))
					address := v.Address
					if address == "" {
						address = strings.ToLower(ins.Metadata.Value("address", ":0"))
					}

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
						"service", v.Service,
					)
				}
			}

			registry.PurgeInstances()
		default:
			// Unsupport
		}
	}

	w.watchPlan = wp

	return w, nil
}

func (w *Watcher) Start() error {
	go w.watchPlan.Run(w.addr)

	return nil
}

func (w *Watcher) Stop() error {
	w.watchPlan.Stop()

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

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
	"context"
	"sync"

	"github.com/hashicorp/consul/api/watch"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
)

// Watcher is a consul component.
type Watcher struct {
	endpoint  string
	watchPlan *watch.Plan
	logger    logger.GeneralLogger

	sync.RWMutex
}

func newWatcher(rg *Consul) (*Watcher, error) {
	w := &Watcher{
		endpoint: rg.config.Endpoint,
		logger:   rg.options.Logger,
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
		// switch data.(type) {
		// case map[string][]string:
		// 	list, err := rg.client.Agent().Services()
		// 	if err != nil {
		// 		rg.options.Logger.ErrorContext(
		// 			rg.ctx,
		// 			"Grab services list failed",
		// 			"registry", rg.String(),
		// 			"id", rg.options.ID,
		// 			"name", rg.options.Name,
		// 			"error", err.Error(),
		// 		)

		// 		return
		// 	}

		// 	for n, v := range list {
		// 		fmt.Println(v)
		// 		if n != "consul" {
		// // Sicky service
		// ins := &registry.Instance{
		// 	ID:               v.ID,
		// 	Service:          v.Service,
		// 	Metadata:         v.Meta,
		// 	Address:          v.Address,
		// 	AdvertiseAddress: v.Address,
		// 	Port:             v.Port,
		// 	AdvertisePort:    v.Port,
		// }
		// // network := strings.ToLower(ins.Metadata.Value("network", "tcp"))
		// // address := v.Address
		// // if address == "" {
		// // 	address = strings.ToLower(ins.Metadata.Value("address", ":0"))
		// // }

		// // switch network {
		// // case "tcp", "tcp4", "tcp6":
		// // 	ins.Address, _ = net.ResolveTCPAddr(network, address)
		// // case "udp", "udp4", "udp6":
		// // 	ins.Address, _ = net.ResolveUDPAddr(network, address)
		// // case "unix", "unixpacket":
		// // 	ins.Address, _ = net.ResolveUnixAddr(network, address)
		// // }
		// registry.RegisterInstance(ins)
		// rg.options.Logger.DebugContext(
		// 	rg.ctx,
		// 	"Consul registry watch event",
		// 	"registry", rg.String(),
		// 	"id", rg.options.ID,
		// 	"name", rg.options.Name,
		// 	"server_id", ins.ID,
		// 	"server_name", ins.Service,
		// 	"server_address", ins.Address,
		// 	"server_port", ins.Port,
		// 	"server_advertise_address", ins.AdvertiseAddress,
		// 	"server_advertise_port", ins.AdvertisePort,
		// )
		//}
		//}

		// registry.PurgeInstances()
		//default:
		// Unsupport
		//}

		// Reload services list
		ins, err := rg.Load()
		if err != nil {
			rg.options.Logger.ErrorContext(
				rg.ctx,
				"reload services list failed",
				"registry", rg.String(),
				"id", rg.options.ID,
				"name", rg.options.Name,
				"error", err.Error(),
			)

			return
		}

		rg.options.Logger.InfoContext(
			rg.ctx,
			"watcher triggered",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)

		registry.PurgePool(ins)
	}

	w.watchPlan = wp

	return w, nil
}

// Start starts the component.
func (w *Watcher) Start() error {
	// Run blocks until Stop. A terminal watch-loop error has nowhere to
	// propagate from a detached goroutine, but it must never vanish
	// silently: log it so operators see the watch died.
	go func() {
		if err := w.watchPlan.Run(w.endpoint); err != nil {
			if w.logger != nil {
				w.logger.ErrorContext(
					context.Background(),
					"Consul watcher run failed",
					"endpoint", w.endpoint,
					"error", err.Error(),
				)
			}
		}
	}()

	return nil
}

// Stop stops the component and releases resources.
func (w *Watcher) Stop() error {
	if w.watchPlan != nil {
		w.watchPlan.Stop()
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

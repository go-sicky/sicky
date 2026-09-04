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
 * @file metrics.go
 * @package metrics
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	pool = make(map[string]prometheus.Collector)
	lock sync.RWMutex
)

var (
	// Server Metrics
	NumGRPCServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_grpc_server_access",
			Help: "Number of grpc access",
		},
	)
	NumHTTPServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_http_server_access",
			Help: "Number of http access",
		},
	)
	NumFiberServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_fiber_server_access",
			Help: "Number of fiber access",
		},
	)
	NumTCPServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_tcp_server_access",
			Help: "Number of tcp access",
		},
	)
	NumUDPServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_udp_server_access",
			Help: "Number of udp access",
		},
	)
	NumWebsocketServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_websocket_server_access",
			Help: "Number of websocket access",
		},
	)

	// Client Metrics
	NumGRPCClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_grpc_client_call",
			Help: "Number of grpc call",
		},
	)
	NumHTTPClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_http_client_call",
			Help: "Number of http call",
		},
	)
	NumTCPClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_tcp_client_call",
			Help: "Number of tcp call",
		},
	)
	NumUDPClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_udp_client_call",
			Help: "Number of udp call",
		},
	)
	NumWebsocketClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_websocket_client_call",
			Help: "Number of websocket call",
		},
	)
)

func Register(name string, c prometheus.Collector) {
	lock.Lock()
	defer lock.Unlock()

	pool[name] = c
}

func Unregister(name string) {
	lock.Lock()
	defer lock.Unlock()

	delete(pool, name)
}

func UnregisterAll() {
	lock.Lock()
	defer lock.Unlock()

	for k := range pool {
		delete(pool, k)
	}
}

func Get(name string) prometheus.Collector {
	lock.RLock()
	defer lock.RUnlock()

	return pool[name]
}

func GetAll() map[string]prometheus.Collector {
	lock.RLock()
	defer lock.RUnlock()

	out := make(map[string]prometheus.Collector, len(pool))
	for k, v := range pool {
		out[k] = v
	}

	return out
}

func init() {
	UnregisterAll()

	Register("num_grpc_server_access", NumGRPCServerAccessCounter)
	Register("num_http_server_access", NumHTTPServerAccessCounter)
	Register("num_fiber_server_access", NumFiberServerAccessCounter)
	Register("num_tcp_server_access", NumTCPServerAccessCounter)
	Register("num_udp_server_access", NumUDPServerAccessCounter)
	Register("num_websocket_server_access", NumWebsocketServerAccessCounter)

	Register("num_grpc_client_call", NumGRPCClientCallCounter)
	Register("num_http_client_call", NumHTTPClientCallCounter)
	Register("num_tcp_client_call", NumTCPClientCallCounter)
	Register("num_udp_client_call", NumUDPClientCallCounter)
	Register("num_websocket_client_call", NumWebsocketClientCallCounter)

	Register("build_info", collectors.NewBuildInfoCollector())
	Register("go_collector", collectors.NewGoCollector())
	Register("process_collector", collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

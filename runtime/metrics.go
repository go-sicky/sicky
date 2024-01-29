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
 * @file metrics.go
 * @package runtime
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package runtime

import "github.com/prometheus/client_golang/prometheus"

var (
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
	NumNatsServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_nats_server_access",
			Help: "Number of nats access",
		},
	)
	NumWebsocketServerAccessCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_websocket_server_access",
			Help: "Number of websocket access",
		},
	)

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
	NumNatsClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_nats_client_call",
			Help: "Number of nats call",
		},
	)
	NumWebsocketClientCallCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "num_websocket_client_call",
			Help: "Number of websocket call",
		},
	)
)

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

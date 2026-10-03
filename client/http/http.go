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
 * @file http.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 12/08/2023
 */

package http

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/tracer"
)

// HTTPClient : Client definition.
type HTTPClient struct {
	config  *Config
	options *client.Options
	ctx     context.Context

	tracer trace.Tracer

	// rt is the RoundTripper used for every request. It is a
	// *http.Client wrapping a dial-overriding transport, so a caller-supplied
	// DialFunc costs nothing extra on the default path.
	rt atomic.Pointer[http.Client]
}

// var (
// 	clients = make(map[string]*HTTPClient, 0)
// )

// func Instance(name string, clt ...*HTTPClient) *HTTPClient {
// 	if len(clt) > 0 {
// 		// Set value
// 		clients[name] = clt[0]

// 		return clt[0]
// 	}

// 	return clients[name]
// }

// New HTTP client.
func New(opts *client.Options, cfg *Config) *HTTPClient {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	clt := &HTTPClient{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	clt.options.Logger.InfoContext(
		clt.ctx,
		"client created",
		"client", clt.String(),
		"id", clt.options.ID,
		"name", clt.options.Name,
	)

	// Snapshot the global tracer (may be nil when tracing disabled).
	if tracer.Default() != nil {
		clt.tracer = tracer.Default().Tracer(clt.Name())
	}

	client.Set(clt)

	return clt
}

// Options returns the runtime options.
func (clt *HTTPClient) Options() *client.Options {
	return clt.options
}

// Context returns the component context.
func (clt *HTTPClient) Context() context.Context {
	return clt.ctx
}

// Connect connects to the backend.
func (clt *HTTPClient) Connect() error {
	return nil
}

// Disconnect disconnects from the backend.
func (clt *HTTPClient) Disconnect() error {
	return nil
}

// Call implements client.Client and always fails: the signature carries no
// target, so it cannot perform a call. It used to return nil, reporting a
// delivery that never happened. Use Invoke/NewStream (gRPC) or Do (HTTP).
func (clt *HTTPClient) Call() error {
	return client.ErrClientNotImplemented
}

// StartSpan starts a client span for an outbound request. The caller must
// propagate the returned ctx via InjectHTTP (or Do, which does both).
//
//nolint:spancheck // span ownership transfers to the caller; Do ends it after the round-trip
func (clt *HTTPClient) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if clt.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	return clt.tracer.Start(ctx, name)
}

// InjectHTTP writes the span context from ctx into outgoing request
// headers (W3C traceparent/tracestate/baggage + B3) and ensures an
// X-Request-ID exists for end-to-end correlation.
func InjectHTTP(ctx context.Context, h http.Header) {
	carrier := propagation.MapCarrier{}
	tracer.Inject(ctx, carrier)
	for k, v := range carrier {
		if v == "" {
			continue
		}

		h.Set(k, v)
	}

	if h.Get("X-Request-ID") == "" {
		h.Set("X-Request-ID", uuid.New().String())
	}
}

// Do sends an outbound HTTP request with tracing. It starts a span named
// "<method> <host>", injects propagation headers, records errors, and
// bumps the client call counter.
func (clt *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return clt.DoWithTransport(req, nil)
}

// DialFunc resolves a request URL host to an address to connect to. It exists
// so a caller (or a test) can route a request whose Host header must stay
// arbitrary — DNS rebinding protection, a fixed upstream, a service mesh.
type DialFunc func(host string) (address string, err error)

// dialTransport routes every connection through dial.
type dialTransport struct {
	dial DialFunc
	base http.RoundTripper
}

func (t *dialTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}

	addr, err := t.dial(req.URL.Host)
	if err != nil {
		return nil, err
	}

	// The Host header must keep the caller's value — that is the point of a
	// DialFunc — so only the dial target is swapped.
	clone := req.Clone(req.Context())
	clone.URL.Host = addr

	return t.base.RoundTrip(clone)
}

// roundTripper returns the client to use, building the dial-overriding one at
// most once per dial function so a hot path does not allocate a transport per
// request.
func (clt *HTTPClient) roundTripper(dial DialFunc) *http.Client {
	if dial == nil {
		return http.DefaultClient
	}

	if c := clt.rt.Load(); c != nil {
		return c
	}

	built := &http.Client{Transport: &dialTransport{dial: dial}}
	if clt.rt.CompareAndSwap(nil, built) {
		return built
	}

	return clt.rt.Load()
}

// DoWithTransport is Do with an optional dial override. A nil dialFunc uses
// the same http.DefaultClient Do uses.
//
// The dial override changes nothing about the metric labels: req.URL.Host is
// still what the caller supplied and is still normalized on its way into the
// counter, so the cardinality bound does not depend on this path being used.
func (clt *HTTPClient) DoWithTransport(req *http.Request, dial DialFunc) (*http.Response, error) {
	start := time.Now()
	ctx := req.Context()
	var span trace.Span
	if clt.tracer != nil {
		spanName := req.Method + " " + req.URL.Host
		ctx, span = clt.tracer.Start(ctx, spanName)
		defer span.End()
		req = req.WithContext(ctx)
	}

	InjectHTTP(ctx, req.Header)

	// G704 (SSRF taint) is accepted here by design: the request URL is the
	// caller's explicit input, and a framework cannot decide for the
	// application which destinations are reachable. An application that
	// proxies untrusted URLs must validate the target itself.
	//nolint:gosec // G704: the request URL is the caller's explicit input by design
	resp, err := clt.roundTripper(dial).Do(req)
	if err != nil && span != nil {
		span.RecordError(err)
	}

	code := "error"
	if err == nil && resp != nil {
		code = strconv.Itoa(resp.StatusCode)
	} else if err != nil {
		metrics.ClientErrorsTotal.WithLabelValues("http", metrics.NormalizeHTTPMethod(req.Method), "do").Inc()
	}

	// Both label values are normalized: the method and the host are supplied
	// by the caller of Do, so a proxy or fetch-a-URL feature would otherwise
	// mint one permanent series per request. The host in particular is why
	// NormalizeClientHost exists — see its doc comment.
	metrics.ObserveClientRequest(
		"http",
		metrics.NormalizeHTTPMethod(req.Method),
		metrics.NormalizeClientHost(req.URL.Host),
		code,
		time.Since(start),
	)

	return resp, err
}

// String returns a human-readable name.
func (clt *HTTPClient) String() string {
	return "http"
}

// Name returns the component name.
func (clt *HTTPClient) Name() string {
	return clt.options.Name
}

// ID returns the unique instance ID.
func (clt *HTTPClient) ID() uuid.UUID {
	return clt.options.ID
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

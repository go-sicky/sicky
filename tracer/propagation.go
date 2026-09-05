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
 * @file propagation.go
 * @package tracer
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package tracer

import (
	"context"
	"strings"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// W3C + B3 dual-track propagator. Extract accepts traceparent/tracestate,
// baggage, B3 single and B3 multi headers; Inject emits W3C + B3 multi so
// standard OTLP backends (Tempo/Jaeger/Collector) and legacy B3 peers
// both stay linked.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader|b3.B3SingleHeader)),
	)
}

// InstallPropagator makes sicky the owner of the global OTEL propagator.
// Call once after a tracer initializes successfully.
func InstallPropagator() {
	otel.SetTextMapPropagator(Propagator())
}

// Extract pulls an upstream span context out of carrier headers.
func Extract(ctx context.Context, c propagation.TextMapCarrier) context.Context {
	return Propagator().Extract(ctx, c)
}

// Inject writes the current span context into carrier headers
// (W3C traceparent/tracestate + baggage + B3, per Propagator).
func Inject(ctx context.Context, c propagation.TextMapCarrier) {
	Propagator().Inject(ctx, c)
}

// maxSanitizedValueLen bounds client-controlled baggage so a 10KB
// request-id cannot blow up logs, spans, or downstream metadata.
const maxSanitizedValueLen = 128

// Sanitize trims, truncates, and charset-filters inbound propagation
// values. Anything outside [A-Za-z0-9-_.:] drops the whole value, so
// forged trace IDs never enter logs or spans.
//
// NOTE: tracestate values legitimately contain '=' / ',' / space and are
// therefore NOT sanitized here — tracestate is never stored, only passed
// through by the OTEL propagator itself. Sanitize applies to request IDs
// and B3 headers only.
func Sanitize(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > maxSanitizedValueLen {
		v = v[:maxSanitizedValueLen]
	}

	for i := range len(v) {
		c := v[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == ':' {
			continue
		}

		return ""
	}

	return v
}

// RedactDSN strips userinfo (tokens) from a DSN/endpoint for safe logging.
// "https://token@host/path" -> "https://host/path".
func RedactDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}

	scheme := ""
	rest := dsn[at+1:]
	if i := strings.Index(dsn[:at], "://"); i >= 0 {
		scheme = dsn[:i+3]
	}

	return scheme + rest
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

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
 * @file logger_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package grpc

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-sicky/sicky/logger"
)

// capturingLogger records the last LogContext call. Only LogContext is
// implemented because that is the single method the access logger writes
// through; the embedded interface is never called.
type capturingLogger struct {
	logger.GeneralLogger

	args []any
}

func (c *capturingLogger) LogContext(_ context.Context, _ logger.Level, _ string, args ...any) {
	c.args = append([]any(nil), args...)
}

// field returns the value logged under key in a fixed-order args slice.
func (c *capturingLogger) field(key string) any {
	for i := 0; i+1 < len(c.args); i += 2 {
		if s, ok := c.args[i].(string); ok && s == key {
			return c.args[i+1]
		}
	}

	return nil
}

// logField runs the access logger over one metadata pair and returns the
// value it logged under logKey.
func logField(t *testing.T, mdKey, mdVal, logKey string) string {
	t.Helper()

	capLog := &capturingLogger{}
	interceptor := NewAccessLoggerInterceptor(LoggerConfig{Logger: capLog})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(mdKey, mdVal))
	info := &grpc.UnaryServerInfo{FullMethod: "/pkg.Service/Method"}

	if _, err := interceptor(ctx, nil, info, func(context.Context, any) (any, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("interceptor: %v", err)
	}

	got, ok := capLog.field(logKey).(string)
	if !ok {
		t.Fatalf("%s missing or not a string in %v", logKey, capLog.args)
	}

	return got
}

// TestAccessLoggerBoundsClientMetadata covers the default configuration,
// where no tracer is configured and the tracing interceptor short-circuits
// before it can sanitize anything. The access logger must sanitize at the
// read site instead, exactly like the HTTP stacks do.
//
// tracer.Sanitize truncates to 128 bytes and *then* charset-filters, so an
// oversized value of legal characters legitimately survives as its first
// 128 bytes: the property under test is the bound, not emptiness.
func TestAccessLoggerBoundsClientMetadata(t *testing.T) {
	// 10 KB of client-controlled metadata: raw it is a single log field
	// big enough to be its own incident, bounded only by gRPC's
	// MaxHeaderListSize.
	oversized := strings.Repeat("a", 10*1024)

	for _, key := range []struct{ md, log string }{
		{"x-request-id", "request-id"},
		{"x-b3-traceid", "trace-id"},
		{"x-b3-spanid", "parent-span-id"},
	} {
		t.Run(key.log, func(t *testing.T) {
			got := logField(t, key.md, oversized, key.log)

			if len(got) > 128 {
				t.Fatalf("%s logged %d bytes of client metadata, want <= 128: the access log must sanitize at the read site, not defer it to the tracing interceptor (which does not run when tracer.type is \"none\")", key.log, len(got))
			}
		})
	}
}

// TestAccessLoggerRejectsForgedMetadata covers the other half of the
// contract: a byte outside [A-Za-z0-9-_.:] drops the whole value, so a
// forged trace id or a newline-smuggled request id never reaches the log.
func TestAccessLoggerRejectsForgedMetadata(t *testing.T) {
	// A newline inside the first 128 bytes, which is where the charset
	// filter runs (truncate first, filter second).
	forged := "abc\nforged=" + strings.Repeat("z", 200)

	for _, key := range []struct{ md, log string }{
		{"x-request-id", "request-id"},
		{"x-b3-traceid", "trace-id"},
		{"x-b3-spanid", "parent-span-id"},
	} {
		t.Run(key.log, func(t *testing.T) {
			got := logField(t, key.md, forged, key.log)

			if got != "" {
				t.Fatalf("%s logged %q, want empty: a byte outside the propagation charset must drop the whole value", key.log, got)
			}
		})
	}
}

// TestAccessLoggerKeepsLegitimateMetadata guards against over-sanitizing: a
// well-formed propagation value must survive, and so must user-agent, which
// the propagation charset would reject for every real client.
func TestAccessLoggerKeepsLegitimateMetadata(t *testing.T) {
	const (
		requestID = "3f8c1b2a-4d5e-6f70-8192-a3b4c5d6e7f8"
		userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) curl/8.4.0"
	)

	if got := logField(t, "x-request-id", requestID, "request-id"); got != requestID {
		t.Fatalf("request-id = %q, want %q: sanitize must not eat a valid value", got, requestID)
	}

	// user-agent carries spaces, slashes and semicolons: it must stay verbatim.
	if got := logField(t, "user-agent", userAgent, "user-agent"); got != userAgent {
		t.Fatalf("user-agent = %q, want %q: user-agent is logged verbatim in all three stacks", got, userAgent)
	}
}

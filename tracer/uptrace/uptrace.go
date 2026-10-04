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
 * @file uptrace.go
 * @package uptrace
 * @author Dr.NP <np@herewe.tech>
 * @since 12/30/2025
 */

package uptrace

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/go-sicky/sicky/tracer"
)

// Two shapes, because the missing one is the common operator mistake.
//
// The scheme form keeps its scheme so the rejection reason stays readable.
var dsnURLUserinfoPattern = regexp.MustCompile(
	`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s"'?#@]+)@`)

// The scheme-less form is what "does not have a scheme" actually reports —
// an operator who typed "TOKEN@host/1" instead of "https://TOKEN@host/1".
// Requiring a dotted host after the "@" keeps it from firing on ordinary
// prose, at the cost of also matching an email address in a diagnostic. That
// trade is deliberate: over-redacting an address costs nothing, and leaking a
// token costs a credential.
var dsnBareUserinfoPattern = regexp.MustCompile(
	`([^/\s"'?#@]+)@([^/\s"'?#@]*\.[^/\s"'?#@]+)`)

// redactUserinfo strips the credential from every URL in a message while
// leaving the rest intact.
//
// utils.RedactDSN would be the wrong tool here: on a string that looks like it
// carries a secret it returns the single word "REDACTED", which would throw
// away the part the operator needs — whether the scheme was missing, the host
// unparseable, or the token absent.
func redactUserinfo(msg string) string {
	msg = dsnURLUserinfoPattern.ReplaceAllString(msg, "${1}REDACTED@")

	return dsnBareUserinfoPattern.ReplaceAllString(msg, "REDACTED@${2}")
}

// dsnRedactingLogger forwards the dependency's diagnostics into sicky's
// logger with the DSN credential removed. It exists because uptrace-go offers
// no other seam: its logger is a package-level variable defaulting to stderr.
type dsnRedactingLogger struct {
	ctx     context.Context
	options *tracer.Options
}

// Printf implements uptrace-go's internal.ILogger.
func (l dsnRedactingLogger) Printf(format string, v ...any) {
	l.options.Logger.WarnContext(
		l.ctx,
		"uptrace client",
		"message", redactUserinfo(fmt.Sprintf(format, v...)),
	)
}

// UptraceTracer is a uptrace component.
type UptraceTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	provider *sdktrace.TracerProvider
}

// New creates a new instance (nil on invalid config).
func New(opts *tracer.Options, cfg *Config) *UptraceTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &UptraceTracer{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	if cfg.DSN == "" {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"uptrace DSN is required",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
		)

		return nil
	}

	// Sampling is controlled server-side by Uptrace; the client-side
	// SampleRate field is deprecated and ignored (kept for config compat).
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		cfg.SampleRate = 1.0
	}

	if cfg.SampleRate != 1.0 {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"uptrace sample_rate is deprecated and ignored; sampling is server-side",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
		)
		cfg.SampleRate = 1.0
	}

	svcName := cfg.ServiceName
	if svcName == "" {
		svcName = opts.Name
	}

	svcVer := cfg.ServiceVersion
	if svcVer == "" {
		svcVer = "latest"
	}

	// Shut down any previous Uptrace-owned provider so re-New() calls
	// (tests, config reload) don't leak the old global provider.
	var shutDown *sdktrace.TracerProvider
	if prev, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok && prev != nil {
		shutDown = prev

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = prev.Shutdown(shutdownCtx)
		// Both shutdowns share a live context: canceling first made the
		// second one return immediately, leaking whatever it owned.
		_ = uptrace.Shutdown(shutdownCtx)
		shutdownCancel()
	}

	// Take over the dependency's diagnostics before handing it the DSN.
	//
	// uptrace-go builds every DSN rejection with the raw input embedded
	// (dsn.go: fmt.Errorf("DSN=%q does not have a host", dsnStr)) and
	// prints it through a package logger wired to os.Stderr at init. The
	// token is the DSN's userinfo, so a typo in the host or the scheme —
	// an ordinary operator mistake — printed the credential in clear to fd
	// 2, outside sicky's logger and past every redaction this package does.
	uptrace.SetLogger(dsnRedactingLogger{ctx: tc.ctx, options: opts})

	// Configure Uptrace (SDK-owned track, independent from standard OTLP).
	uptrace.ConfigureOpentelemetry(
		uptrace.WithDSN(cfg.DSN),
		uptrace.WithServiceName(svcName),
		uptrace.WithServiceVersion(svcVer),
	)

	// Get the TracerProvider configured by Uptrace.
	//
	// ConfigureOpentelemetry returns early on a bad DSN without installing a
	// provider, so the global still points at the one just shut down above and
	// the type assertion below would succeed — handing back a tracer that
	// looks healthy and records nothing. Comparing against what was there
	// before is what distinguishes "configured" from "left the corpse".
	provider, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok || provider == shutDown {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"failed to get TracerProvider from Uptrace",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"dsn", tracer.RedactDSN(cfg.DSN),
		)

		return nil
	}

	tc.provider = provider

	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer created",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tracer.RedactDSN(cfg.DSN),
		"service", svcName,
		"version", svcVer,
	)
	tracer.Set(tc)

	return tc
}

// Context returns the component context.
func (tc *UptraceTracer) Context() context.Context {
	return tc.ctx
}

// Options returns the runtime options.
func (tc *UptraceTracer) Options() *tracer.Options {
	return tc.options
}

// String returns a human-readable name.
func (tc *UptraceTracer) String() string {
	return "uptrace"
}

// ID returns the unique instance ID.
func (tc *UptraceTracer) ID() uuid.UUID {
	return tc.options.ID
}

// Name returns the component name.
func (tc *UptraceTracer) Name() string {
	return tc.options.Name
}

// Start starts the component.
func (tc *UptraceTracer) Start() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer started",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tracer.RedactDSN(tc.config.DSN),
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
	)

	return nil
}

// Stop stops the component and releases resources.
func (tc *UptraceTracer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if tc.provider != nil {
		if err := tc.provider.Shutdown(ctx); err != nil {
			tc.options.Logger.ErrorContext(
				tc.ctx,
				"tracer provider shutdown failed",
				"tracer", tc.String(),
				"id", tc.options.ID,
				"name", tc.options.Name,
				"dsn", tracer.RedactDSN(tc.config.DSN),
				"error", err.Error(),
			)

			return err
		}
	}

	// Shutdown Uptrace
	if err := uptrace.Shutdown(ctx); err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"uptrace shutdown failed",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"dsn", tracer.RedactDSN(tc.config.DSN),
			"error", err.Error(),
		)

		return err
	}

	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer stopped",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tracer.RedactDSN(tc.config.DSN),
	)

	return nil
}

// Provider returns the provider.
func (tc *UptraceTracer) Provider() *sdktrace.TracerProvider {
	return tc.provider
}

// Tracer returns the tracer.
func (tc *UptraceTracer) Tracer(name string) trace.Tracer {
	if tc.provider == nil {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"requested tracer from nil provider",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"dsn", tracer.RedactDSN(tc.config.DSN),
		)

		return noop.NewTracerProvider().Tracer(name)
	}

	tc.options.Logger.DebugContext(
		tc.ctx,
		"requested tracer",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tracer.RedactDSN(tc.config.DSN),
		"request", name,
	)

	return tc.provider.Tracer(name)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

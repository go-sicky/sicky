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
 * @file runtime.go
 * @package runtime
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package runtime

import (
	"github.com/go-sicky/sicky/logger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Lock-less maps here
var (
	loggers        = make(map[string]*logger.GeneralLogger, 0)
	traceProviders = make(map[string]*sdktrace.TracerProvider, 0)
	tracers        = make(map[string]trace.Tracer, 0)
)

func Logger(name string, l ...*logger.GeneralLogger) *logger.GeneralLogger {
	if len(l) > 0 {
		// Set value
		loggers[name] = l[0]

		return l[0]
	}

	return loggers[name]
}

func TraceProvider(name string, tp ...*sdktrace.TracerProvider) *sdktrace.TracerProvider {
	if len(tp) > 0 {
		// Set value
		traceProviders[name] = tp[0]

		return tp[0]
	}

	return traceProviders[name]
}

func Tracer(name string, t ...trace.Tracer) trace.Tracer {
	if len(t) > 0 {
		// Set value
		tracers[name] = t[0]

		return t[0]
	}

	return tracers[name]
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

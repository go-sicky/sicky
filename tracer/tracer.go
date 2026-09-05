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
 * @file tracer.go
 * @package tracer
 * @author Dr.NP <np@herewe.tech>
 * @since 09/14/2024
 */

package tracer

import (
	"context"
	"maps"
	"sync"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Tracer : tracer abstraction.
type Tracer interface {
	// Get context
	Context() context.Context
	// Server options
	Options() *Options
	// Stringify
	String() string
	// Tracer ID
	ID() uuid.UUID
	// Tracer name
	Name() string
	// Start tracer
	Start() error
	// Stop tracer
	Stop() error
	// Trace provider
	Provider() *sdktrace.TracerProvider
	// Get tracer
	Tracer(name string) trace.Tracer
}

var (
	tracers       = make(map[uuid.UUID]Tracer)
	defaultTracer Tracer
	trMu          sync.RWMutex
)

// Set registers tracer instances; the first one becomes the default.
func Set(trs ...Tracer) {
	trMu.Lock()
	defer trMu.Unlock()

	for _, trc := range trs {
		tracers[trc.ID()] = trc
		if defaultTracer == nil {
			defaultTracer = trc
		}
	}
}

// Get looks up a tracer instance by ID.
func Get(id uuid.UUID) Tracer {
	trMu.RLock()
	defer trMu.RUnlock()

	return tracers[id]
}

// Default returns the default tracer instance.
func Default() Tracer {
	trMu.RLock()
	defer trMu.RUnlock()

	return defaultTracer
}

// Tracers returns the managed tracers.
func Tracers() map[uuid.UUID]Tracer {
	trMu.RLock()
	defer trMu.RUnlock()

	out := make(map[uuid.UUID]Tracer, len(tracers))
	maps.Copy(out, tracers)

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	trMu.Lock()
	defer trMu.Unlock()

	tracers = make(map[uuid.UUID]Tracer)
	defaultTracer = nil
}

/* {{{ [Helpers]. */
func Provider() *sdktrace.TracerProvider {
	trMu.RLock()
	defer trMu.RUnlock()

	if defaultTracer == nil {
		return nil
	}

	return defaultTracer.Provider()
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

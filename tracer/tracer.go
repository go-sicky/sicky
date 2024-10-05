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
	"errors"

	"github.com/google/uuid"
)

// Tracer : tracer abstraction
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
	// Trace context
	Trace() error
}

var (
	tracers = make(map[uuid.UUID]Tracer)
)

func Instance(id uuid.UUID, tracer ...Tracer) Tracer {
	if len(tracer) > 0 {
		tracers[id] = tracer[0]

		return tracer[0]
	}

	return tracers[id]
}

func Trace() error {
	var (
		errs error
	)

	for _, tr := range tracers {
		if tr != nil {
			err := tr.Trace()
			if err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

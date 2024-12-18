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
 * @file runner.go
 * @package runner
 * @author Dr.NP <np@herewe.tech>
 * @since 12/18/2024
 */

package runner

import (
	"context"

	"github.com/google/uuid"
)

type Runner interface {
	// Get context
	Context() context.Context
	// Runner options
	Options() *Options
	// Stringify
	String() string
	// Runner ID
	ID() uuid.UUID
	// Runner name
	Name() string
	// Start runner
	Start() error
	// Stop runner
	Stop() error
	// Consume task
	Task(*Task)
}

type Task struct {
	ID   uuid.UUID
	Data any
}

var (
	runners = make(map[uuid.UUID]Runner)
)

func Instance(id uuid.UUID, runner ...Runner) Runner {
	if len(runner) > 0 {
		runners[id] = runner[0]

		return runner[0]
	}

	return runners[id]
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

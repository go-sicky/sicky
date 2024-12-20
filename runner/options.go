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
 * @file options.go
 * @package runner
 * @author Dr.NP <np@herewe.tech>
 * @since 12/18/2024
 */

package runner

import (
	"runtime"

	"github.com/go-sicky/sicky/logger"
	"github.com/google/uuid"
)

const (
	DefaultBufferSize = 256
)

type Options struct {
	Name       string
	ID         uuid.UUID
	Logger     logger.GeneralLogger
	NThreads   int
	BufferSize int

	Handler func(*Task) error
}

func (o *Options) Ensure() *Options {
	if o == nil {
		o = new(Options)
	}

	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}

	if o.Name == "" {
		o.Name = "Runner::" + o.ID.String()
	}

	if o.Logger == nil {
		o.Logger = logger.DefaultGeneralLogger
	}

	if o.NThreads <= 0 {
		// Default : 4 times of NumCPU()
		o.NThreads = runtime.NumCPU() * 4
	}

	if o.BufferSize < 0 {
		// Default : 256
		o.BufferSize = DefaultBufferSize
	}

	return o
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

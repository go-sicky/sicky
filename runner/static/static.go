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
 * @file static.go
 * @package static
 * @author Dr.NP <np@herewe.tech>
 * @since 12/18/2024
 */

package static

import (
	"context"
	"sync"

	"github.com/go-sicky/sicky/runner"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type Static struct {
	config  *Config
	ctx     context.Context
	options *runner.Options

	wg   sync.WaitGroup
	task chan *runner.Task
}

// New static runner (pool)
func New(opts *runner.Options, cfg *Config) *Static {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	r := &Static{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		task:    make(chan *runner.Task, opts.BufferSize),
	}

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner created",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
	)

	runner.Set(r)

	return r
}

func (r *Static) Context() context.Context {
	return r.ctx
}

func (r *Static) Options() *runner.Options {
	return r.options
}

func (r *Static) String() string {
	return "static"
}

func (r *Static) ID() uuid.UUID {
	return r.options.ID
}

func (r *Static) Name() string {
	return r.options.Name
}

func (r *Static) Start() error {
	// Goroutines
	go func() {
		for idx := 0; idx < r.options.NThreads; idx++ {
			go r._worker()
		}

		r.wg.Wait()
	}()

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner started",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
		"threads", r.options.NThreads,
	)
	return nil
}

func (r *Static) Stop() error {
	if r.task != nil {
		close(r.task)

		r.task = nil
	}

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner stopped",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
	)

	return nil
}

func (r *Static) Task(t *runner.Task) {
	if r.task != nil {
		if t.ID == uuid.Nil {
			t.ID = uuid.New()
		}

		r.task <- t
	}
}

func (r *Static) _worker() error {
	r.wg.Add(1)
	defer r.wg.Done()
	self := utils.GoroutineID()

	for t := range r.task {
		r.options.Logger.TraceContext(
			r.ctx,
			"Runner task created",
			"runner", r.String(),
			"id", r.options.ID,
			"name", r.options.Name,
			"worker", self,
			"task", t.ID.String(),
		)

		// Call handler
		if r.options.Handler != nil {
			err := r.options.Handler(t)
			if err != nil {
				r.options.Logger.ErrorContext(
					r.ctx,
					"Runner worker run failed",
					"runner", r.String(),
					"id", r.options.ID,
					"name", r.options.Name,
					"worker", self,
					"task", t.ID.String(),
					"error", err.Error(),
				)
			}
		}
	}

	r.options.Logger.DebugContext(
		r.ctx,
		"Runner worker created",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
		"worker", self,
	)

	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

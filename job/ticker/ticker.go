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
 * @file ticker.go
 * @package ticker
 * @author Dr.NP <np@herewe.tech>
 * @since 12/25/2025
 */

package ticker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-sicky/sicky/job"
	"github.com/google/uuid"
)

type Ticker struct {
	config  *Config
	ctx     context.Context
	options *job.Options
	ticker  *time.Ticker
	counter atomic.Uint64
	running bool

	tasks []*Task
	sync.RWMutex
}

// New ticker job schedular
func New(opts *job.Options, cfg *Config) *Ticker {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	j := &Ticker{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		running: false,
		tasks:   make([]*Task, 0),
	}

	j.options.Logger.InfoContext(
		j.ctx,
		"Job created",
		"job", j.String(),
		"id", j.options.ID,
		"name", j.options.Name,
	)

	job.Set(j)

	return j
}

func (job *Ticker) Context() context.Context {
	return job.ctx
}

func (job *Ticker) Options() *job.Options {
	return job.options
}

func (job *Ticker) String() string {
	return "ticker"
}

func (job *Ticker) ID() uuid.UUID {
	return job.options.ID
}

func (job *Ticker) Name() string {
	return job.options.Name
}

func (job *Ticker) Add(task *Task) error {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	job.Lock()
	defer job.Unlock()

	job.tasks = append(job.tasks, task)

	return nil
}

func (job *Ticker) Start() error {
	job.Lock()
	defer job.Unlock()

	if job.running {
		return nil
	}

	job.ticker = time.NewTicker(time.Duration(job.config.Interval) * time.Second)
	go func() {
		for t := range job.ticker.C {
			for _, hdl := range job.tasks {
				if job.counter.Load()%hdl.Inteval == 0 {
					err := hdl.Handler(t, job.counter.Load())
					if err != nil {
						job.options.Logger.ErrorContext(
							job.ctx,
							"Ticker handler failed",
							"error", err.Error(),
						)
					} else {
						job.options.Logger.DebugContext(
							job.ctx,
							"Ticker handler success",
							"job", job.String(),
							"handler", hdl.ID,
							"id", job.options.ID,
							"name", job.options.Name,
							"counter", job.counter.Load(),
						)
					}
				}
			}

			// Increase counter
			job.counter.Add(1)
		}
	}()

	job.running = true

	return nil
}

func (job *Ticker) Stop() error {
	job.Lock()
	defer job.Unlock()

	if !job.running {
		return nil
	}

	job.ticker.Stop()
	job.running = false

	return nil
}

/* {{{ [Task] */

type TickerHandler func(time.Time, uint64) error

type Task struct {
	ID      uuid.UUID
	Inteval uint64
	Handler TickerHandler
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

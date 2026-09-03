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
	"errors"
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
	done    chan struct{}
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
	if task == nil {
		return errors.New("ticker task is nil")
	}
	job.Lock()
	defer job.Unlock()

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	if task.Inteval <= 0 {
		task.Inteval = 1
	}

	job.tasks = append(job.tasks, task)

	return nil
}

func (job *Ticker) Start() error {
	job.Lock()
	defer job.Unlock()

	if job.running {
		return nil
	}

	job.done = make(chan struct{})
	job.ticker = time.NewTicker(time.Duration(job.config.Interval) * time.Second)
	go func() {
		for {
			select {
			case t, ok := <-job.ticker.C:
				if !ok {
					return
				}
				job.RLock()
				snapshot := append([]*Task(nil), job.tasks...)
				job.RUnlock()
				count := job.counter.Load()
				for _, hdl := range snapshot {
					if hdl == nil || hdl.Handler == nil || hdl.Inteval <= 0 {
						continue
					}
					if count%hdl.Inteval == 0 {
						func() {
							defer func() {
								_ = recover()
							}()
							err := hdl.Handler(t, count)
							if err != nil {
								job.options.Logger.ErrorContext(
									job.ctx,
									"Ticker handler failed",
									"error", err.Error(),
								)
							}
						}()
					}
				}

				// Increase counter
				job.counter.Add(1)
			case <-job.done:
				return
			}
		}
	}()

	job.running = true

	job.options.Logger.InfoContext(
		job.ctx,
		"Ticker job started",
		"job", job.String(),
		"id", job.options.ID,
		"name", job.options.Name,
		"interval", job.config.Interval,
		"tasks", len(job.tasks),
	)

	return nil
}

func (job *Ticker) Stop() error {
	job.Lock()
	defer job.Unlock()

	if !job.running {
		return nil
	}

	close(job.done)
	job.ticker.Stop()
	job.running = false

	job.options.Logger.InfoContext(
		job.ctx,
		"Ticker job stopped",
		"job", job.String(),
		"id", job.options.ID,
		"name", job.options.Name,
	)

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

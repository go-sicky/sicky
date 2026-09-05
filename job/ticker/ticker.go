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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/metrics"
)

// Ticker is a ticker component.
type Ticker struct {
	config  *Config
	ctx     context.Context
	options *job.Options
	ticker  *time.Ticker
	done    chan struct{}
	counter atomic.Uint64
	running bool
	wg      sync.WaitGroup

	tasks []*Task
	sync.RWMutex
}

// New ticker job schedular.
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

// Context returns the component context.
func (job *Ticker) Context() context.Context {
	return job.ctx
}

// Options returns the runtime options.
func (job *Ticker) Options() *job.Options {
	return job.options
}

// String returns a human-readable name.
func (job *Ticker) String() string {
	return "ticker"
}

// ID returns the unique instance ID.
func (job *Ticker) ID() uuid.UUID {
	return job.options.ID
}

// Name returns the component name.
func (job *Ticker) Name() string {
	return job.options.Name
}

// Add is part of the public API.
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
	metrics.JobRegisteredTotal.WithLabelValues("ticker", "ok").Inc()

	return nil
}

// Start starts the component.
func (job *Ticker) Start() error {
	job.Lock()
	defer job.Unlock()

	if job.running {
		return nil
	}

	job.done = make(chan struct{})
	job.ticker = time.NewTicker(time.Duration(job.config.Interval) * time.Second)
	job.wg.Go(func() {
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
						metrics.JobTicksTotal.WithLabelValues("ticker", "fired").Inc()
						func() {
							start := time.Now()
							taskID := hdl.ID.String()
							result := "ok"
							defer func() {
								if rec := recover(); rec != nil {
									result = "panic"
									job.options.Logger.ErrorContext(
										job.ctx,
										"Ticker handler panicked",
										"job", job.String(),
										"id", job.options.ID,
										"name", job.options.Name,
										"panic", rec,
									)
								}
								metrics.ObserveJobRun("ticker", taskID, result, time.Since(start))
							}()
							err := job.runWithTimeout(hdl, t, count)
							if err != nil {
								result = metrics.ResultOf(err)
								if strings.Contains(err.Error(), "timed out") {
									result = "timeout"
								} else if strings.Contains(err.Error(), "panicked") {
									result = "panic"
								}
								job.options.Logger.ErrorContext(
									job.ctx,
									"Ticker handler failed",
									"error", err.Error(),
								)
							}
						}()
					} else {
						metrics.JobTicksTotal.WithLabelValues("ticker", "skipped").Inc()
					}
				}

				// Increase counter
				job.counter.Add(1)
			case <-job.done:
				return
			}
		}
	})

	job.running = true
	metrics.JobRunning.WithLabelValues("ticker").Set(1)

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

// Stop stops the component and releases resources.
func (job *Ticker) Stop() error {
	job.Lock()
	if !job.running {
		job.Unlock()

		return nil
	}

	close(job.done)
	job.ticker.Stop()
	job.running = false
	metrics.JobRunning.WithLabelValues("ticker").Set(0)
	job.Unlock()

	// Wait for the loop goroutine so Start-Stop-Start cannot double-run.
	job.wg.Wait()

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

// TickerHandler is a ticker component.
type TickerHandler func(time.Time, uint64) error

// Task is a ticker component.
type Task struct {
	ID      uuid.UUID
	Inteval uint64
	Handler TickerHandler
	// Timeout bounds one run. Zero disables. On expiry the tick is
	// reported as failed but the handler keeps running in background
	// (it cannot be killed) and later ticks may overlap it.
	Timeout time.Duration
}

// runWithTimeout executes the task handler with the timeout watchdog.
func (job *Ticker) runWithTimeout(hdl *Task, t time.Time, count uint64) error {
	if hdl == nil || hdl.Handler == nil || hdl.Timeout <= 0 {
		if hdl == nil || hdl.Handler == nil {
			return nil
		}

		return hdl.Handler(t, count)
	}

	timeout := hdl.Timeout

	done := make(chan error, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				job.options.Logger.ErrorContext(
					job.ctx,
					"Ticker task panicked",
					"job", job.String(),
					"id", job.options.ID,
					"name", job.options.Name,
					"panic", rec,
				)
				done <- fmt.Errorf("ticker task panicked: %v", rec)
			}
		}()
		done <- hdl.Handler(t, count)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		job.options.Logger.ErrorContext(
			job.ctx,
			"Ticker task timed out; leaked run continues in background",
			"job", job.String(),
			"id", job.options.ID,
			"name", job.options.Name,
			"timeout", timeout.String(),
		)

		return fmt.Errorf("ticker task timed out after %s", timeout.String())
	}
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

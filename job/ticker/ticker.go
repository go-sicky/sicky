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
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	jobpkg "github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/utils"
)

// Ticker is a ticker component.
type Ticker struct {
	config  *Config
	ctx     context.Context
	options *jobpkg.Options
	ticker  *time.Ticker
	done    chan struct{}
	counter atomic.Uint64
	running bool
	// draining is set when Stop gave up waiting for the loop: Start must
	// refuse to spawn a second one over the still-running first.
	draining atomic.Bool
	wg       sync.WaitGroup

	tasks []*Task
	sync.RWMutex
}

// New ticker job schedular.
func New(opts *jobpkg.Options, cfg *Config) *Ticker {
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
		"job created",
		"job", j.String(),
		"id", j.options.ID,
		"name", j.options.Name,
	)

	jobpkg.Set(j)

	return j
}

// Context returns the component context.
func (job *Ticker) Context() context.Context {
	return job.ctx
}

// Options returns the runtime options.
func (job *Ticker) Options() *jobpkg.Options {
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

	if job.draining.Load() {
		// The previous Stop timed out: the loop is still running.
		return utils.ErrStopTimeout
	}

	// Snapshot both channels into locals. The loop goroutine must close
	// over them rather than read job.ticker/job.done on every iteration:
	// Stop writes those fields under the write lock and a concurrent Start
	// may replace them, so a field read here is an unsynchronised race
	// that also silently hands the old loop the *new* ticker's channel
	// and the *new* done channel, losing the first Stop's close and
	// running the same tasks in two loops.
	done := make(chan struct{})
	ticker := time.NewTicker(time.Duration(job.config.Interval) * time.Second)
	job.done = done
	job.ticker = ticker
	job.wg.Go(func() {
		for {
			select {
			case t, ok := <-ticker.C:
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
							result := jobpkg.ResultOK
							defer func() {
								if rec := recover(); rec != nil {
									result = jobpkg.ResultPanic
									job.options.Logger.ErrorContext(
										job.ctx,
										"ticker handler panicked",
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
								result = jobpkg.Classify(err)

								job.options.Logger.ErrorContext(
									job.ctx,
									"ticker handler failed",
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
			case <-done:
				return
			}
		}
	})

	job.running = true
	metrics.JobRunning.WithLabelValues("ticker").Set(1)

	job.options.Logger.InfoContext(
		job.ctx,
		"ticker job started",
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
	// Bounded: a handler that never returns must not wedge shutdown, and
	// Start stays refused until the loop actually exits.
	job.draining.Store(true)
	if !utils.WaitGroupTimeout(&job.wg, utils.StopTimeout, func() { job.draining.Store(false) }) {
		job.options.Logger.WarnContext(
			job.ctx,
			"ticker stop timed out; the loop is still running",
			"job", job.String(),
			"id", job.options.ID,
			"name", job.options.Name,
			"timeout", utils.StopTimeout.String(),
		)

		return utils.ErrStopTimeout
	}

	job.options.Logger.InfoContext(
		job.ctx,
		"ticker job stopped",
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
					"ticker task panicked",
					"job", job.String(),
					"id", job.options.ID,
					"name", job.options.Name,
					"panic", rec,
				)
				done <- fmt.Errorf("%w: %v", jobpkg.ErrTaskPanic, rec)
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
			"ticker task timed out; leaked run continues in background",
			"job", job.String(),
			"id", job.options.ID,
			"name", job.options.Name,
			"timeout", timeout.String(),
		)

		return fmt.Errorf("%w after %s", jobpkg.ErrTaskTimeout, timeout.String())
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

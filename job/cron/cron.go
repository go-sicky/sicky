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
 * @file cron.go
 * @package cron
 * @author Dr.NP <np@herewe.tech>
 * @since 08/18/2024
 */

package cron

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/metrics"
)

// Cron is a cron component.
type Cron struct {
	config    *Config
	ctx       context.Context
	options   *job.Options
	running   bool
	tasks     []*Task
	scheduler gocron.Scheduler

	sync.RWMutex
}

// New cron job schedular.
func New(opts *job.Options, cfg *Config) *Cron {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	j := &Cron{
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
func (job *Cron) Context() context.Context {
	return job.ctx
}

// Options returns the runtime options.
func (job *Cron) Options() *job.Options {
	return job.options
}

// String returns a human-readable name.
func (job *Cron) String() string {
	return "cron"
}

// ID returns the unique instance ID.
func (job *Cron) ID() uuid.UUID {
	return job.options.ID
}

// Name returns the component name.
func (job *Cron) Name() string {
	return job.options.Name
}

// Add is part of the public API.
func (job *Cron) Add(task *Task) error {
	job.Lock()
	defer job.Unlock()

	if task == nil {
		return errors.New("cron task is nil")
	}

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	job.tasks = append(job.tasks, task)

	if job.running && job.scheduler != nil && task.Handler != nil {
		if _, err := job.scheduler.NewJob(
			gocron.CronJob(task.Expression, true),
			gocron.NewTask(job.runWithTimeout(task, task.Handler)),
		); err != nil {
			metrics.JobRegisteredTotal.WithLabelValues("cron", "error").Inc()

			return err
		}
	}

	metrics.JobRegisteredTotal.WithLabelValues("cron", "ok").Inc()

	return nil
}

// Start starts the component.
func (job *Cron) Start() error {
	job.Lock()
	defer job.Unlock()

	if job.running {
		return nil
	}

	sch, err := gocron.NewScheduler()
	if err != nil {
		return err
	}

	job.scheduler = sch

	// Register all pre-added tasks
	for _, task := range job.tasks {
		if task == nil || task.Handler == nil {
			continue
		}

		_, err := job.scheduler.NewJob(
			gocron.CronJob(task.Expression, true),
			gocron.NewTask(
				job.runWithTimeout(task, task.Handler),
			),
		)
		if err != nil {
			metrics.JobRegisteredTotal.WithLabelValues("cron", "error").Inc()
			job.options.Logger.ErrorContext(
				job.ctx,
				"Register cron task failed",
				"job", job.String(),
				"id", job.options.ID,
				"name", job.options.Name,
				"task_id", task.ID.String(),
				"error", err.Error(),
			)
		} else {
			metrics.JobRegisteredTotal.WithLabelValues("cron", "ok").Inc()
		}
	}

	job.scheduler.Start()

	job.running = true
	metrics.JobRunning.WithLabelValues("cron").Set(1)

	job.options.Logger.InfoContext(
		job.ctx,
		"Cron job started",
		"job", job.String(),
		"id", job.options.ID,
		"name", job.options.Name,
		"tasks", len(job.tasks),
	)

	return nil
}

// Stop stops the component and releases resources.
func (job *Cron) Stop() error {
	job.Lock()
	defer job.Unlock()

	if !job.running {
		return nil
	}

	err := job.scheduler.Shutdown()
	if err != nil {
		return err
	}

	job.running = false
	metrics.JobRunning.WithLabelValues("cron").Set(0)

	return nil
}

/* {{{ [Task]. */
type CronHandler func() error

// Task is a cron component.
type Task struct {
	ID         uuid.UUID
	Expression string
	Handler    CronHandler
	// Timeout bounds one run. Zero disables. On expiry the run is
	// reported as failed but the handler goroutine cannot be killed —
	// it leaks until it returns, and overlapping runs may pile up.
	// Prefer short, idempotent handlers.
	Timeout time.Duration
}

// runWithTimeout executes h with the task timeout watchdog.
func (job *Cron) runWithTimeout(task *Task, h CronHandler) CronHandler {
	if task == nil || h == nil {
		return h
	}

	taskID := task.ID.String()

	if task.Timeout <= 0 {
		// No watchdog: still count runs. Panics propagate to the
		// scheduler unchanged after being recorded.
		return func() (err error) {
			start := time.Now()
			defer func() {
				if r := recover(); r != nil {
					metrics.ObserveJobRun("cron", taskID, "panic", time.Since(start))

					panic(r)
				}
			}()

			err = h()
			metrics.ObserveJobRun("cron", taskID, metrics.ResultOf(err), time.Since(start))

			return err
		}
	}

	timeout := task.Timeout

	return func() error {
		start := time.Now()
		done := make(chan error, 1)
		go func() {
			// A panicking handler must not leave the watchdog hanging
			// forever with an invisible failure.
			defer func() {
				if rec := recover(); rec != nil {
					job.options.Logger.ErrorContext(
						job.ctx,
						"Cron task panicked",
						"job", job.String(),
						"id", job.options.ID,
						"name", job.options.Name,
						"task_id", task.ID.String(),
						"panic", rec,
					)
					done <- fmt.Errorf("cron task %s panicked: %v", task.ID.String(), rec)
				}
			}()

			done <- h()
		}()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-done:
			result := metrics.ResultOf(err)
			if err != nil && strings.Contains(err.Error(), "panicked") {
				result = "panic"
			}
			metrics.ObserveJobRun("cron", taskID, result, time.Since(start))

			return err
		case <-timer.C:
			job.options.Logger.ErrorContext(
				job.ctx,
				"Cron task timed out; leaked run continues in background",
				"job", job.String(),
				"id", job.options.ID,
				"name", job.options.Name,
				"task_id", task.ID.String(),
				"timeout", timeout.String(),
			)
			metrics.ObserveJobRun("cron", taskID, "timeout", time.Since(start))

			return fmt.Errorf("cron task %s timed out after %s", task.ID.String(), timeout.String())
		}
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

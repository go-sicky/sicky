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
	"sync"

	"github.com/go-co-op/gocron/v2"
	"github.com/go-sicky/sicky/job"
	"github.com/google/uuid"
)

type Cron struct {
	config    *Config
	ctx       context.Context
	options   *job.Options
	running   bool
	tasks     []gocron.Job
	scheduler gocron.Scheduler

	sync.RWMutex
}

// New cron job schedular
func New(opts *job.Options, cfg *Config) *Cron {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	j := &Cron{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		running: false,
		tasks:   make([]gocron.Job, 0),
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

func (job *Cron) Context() context.Context {
	return job.ctx
}

func (job *Cron) Options() *job.Options {
	return job.options
}

func (job *Cron) String() string {
	return "cron"
}

func (job *Cron) ID() uuid.UUID {
	return job.options.ID
}

func (job *Cron) Name() string {
	return job.options.Name
}

func (job *Cron) Add(task *Task) error {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	j, err := job.scheduler.NewJob(
		gocron.CronJob(task.Expression, true),
		gocron.NewTask(
			task.Handler,
		),
	)
	if err != nil {
		return err
	}

	job.tasks = append(job.tasks, j)

	return nil
}

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
	job.running = true

	return nil
}

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

	return nil
}

/* {{{ [Task] */
type CronHandler func() error

type Task struct {
	ID         uuid.UUID
	Expression string
	Handler    CronHandler
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

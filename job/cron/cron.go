/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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

	"github.com/go-sicky/sicky/job"
)

type Cron struct {
	config  *Config
	ctx     context.Context
	options *job.Options
}

// New cron job schedular
func New(opts *job.Options, cfg *Config) *Cron {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	job := &Cron{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	job.options.Logger.InfoContext(
		job.ctx,
		"Job created",
		"job", job.String(),
		"id", job.options.ID,
		"name", job.options.Name,
	)

	return job
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

func (job *Cron) Start() error {
	return nil
}

func (job *Cron) Stop() error {
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

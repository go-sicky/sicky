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
 * @file nsq.go
 * @package nsq
 * @author Dr.NP <np@herewe.tech>
 * @since 08/14/2024
 */

package nsq

import (
	"context"

	"github.com/go-sicky/sicky/broker"
	"github.com/google/uuid"
)

type Nsq struct {
	config  *Config
	ctx     context.Context
	options *broker.Options
}

func New(opts *broker.Options, cfg *Config) *Nsq {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	brk := &Nsq{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Broker created",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	broker.Instance(opts.ID, brk)

	return brk
}

func (brk *Nsq) Context() context.Context {
	return brk.ctx
}

func (brk *Nsq) Options() *broker.Options {
	return brk.options
}

func (brk *Nsq) String() string {
	return "nsq"
}

func (brk *Nsq) ID() uuid.UUID {
	return brk.options.ID
}

func (brk *Nsq) Name() string {
	return brk.options.Name
}

func (brk *Nsq) Connect() error {
	return nil
}

func (brk *Nsq) Disconnect() error {
	return nil
}

func (brk *Nsq) Publish(topic string, m *broker.Message) error {
	return nil
}

func (brk *Nsq) Subscribe(topic string, h broker.Handler) error {
	return nil
}

func (brk *Nsq) Unsubscribe(topic string) error {
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

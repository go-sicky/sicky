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
 * @file nats.go
 * @package nats
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package nats

import (
	"context"
	"errors"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/utils"
	"github.com/nats-io/nats.go"
)

type Nats struct {
	config  *Config
	ctx     context.Context
	options *broker.Options
	conn    *nats.Conn

	subscriptions map[string]*nats.Subscription
}

func New(opts *broker.Options, cfg *Config) *Nats {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	brk := &Nats{
		config:        cfg,
		ctx:           context.Background(),
		options:       opts,
		subscriptions: make(map[string]*nats.Subscription),
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Broker created",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	return brk
}

func (brk *Nats) Context() context.Context {
	return brk.ctx
}

func (brk *Nats) Options() *broker.Options {
	return brk.options
}

func (brk *Nats) String() string {
	return "nats"
}

func (brk *Nats) Connect() error {
	nc, err := nats.Connect(
		brk.config.URL,
	)
	if err != nil {
		brk.options.Logger.ErrorContext(brk.ctx, "broker connect failed", "error", err.Error())

		return err
	}

	brk.conn = nc

	return nil
}

func (brk *Nats) Disconnect() error {
	if brk.conn != nil && !brk.conn.IsClosed() {
		brk.conn.Close()
		brk.conn = nil
	}

	return nil
}

func (brk *Nats) Publish(topic string, m *broker.Message) error {
	if brk.conn == nil || !brk.conn.IsConnected() {
		return errors.New("broker not connected")
	}

	msg := nats.NewMsg(topic)
	if m != nil {
		for k, v := range m.Header() {
			msg.Header.Add(k, v)
		}

		msg.Data = m.Body()
	}

	return brk.conn.PublishMsg(msg)
}

func (brk *Nats) Subscribe(topic string, h broker.Handler) error {
	if brk.conn == nil || !brk.conn.IsConnected() {
		return errors.New("broker not connected")
	}

	if brk.subscriptions[topic] != nil {
		return errors.New("topic already subscribed")
	}

	sub, err := brk.conn.Subscribe(topic, func(msg *nats.Msg) {
		if h != nil {
			m := &broker.Message{}
			hdr := utils.NewMetadata()
			for k, vs := range msg.Header {
				if len(vs) > 0 {
					hdr.Set(k, vs[0])
				}
			}

			m.Header(hdr)
			m.Body(msg.Data)

			err := h(m)
			if err != nil {
				brk.options.Logger.ErrorContext(brk.ctx, "broker handler error", "error", err.Error())
			}
		}
	})

	if err != nil {
		brk.options.Logger.ErrorContext(brk.ctx, "broker subscribe failed", "error", err.Error())

		return err
	}

	brk.subscriptions[topic] = sub

	return nil
}

func (brk *Nats) Unsubscribe(topic string) error {
	sub := brk.subscriptions[topic]
	if sub != nil {
		sub.Unsubscribe()
	}

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

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
 * @file nats.go
 * @package nats
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package nats

import (
	"context"
	"errors"

	"maps"

	"github.com/go-sicky/sicky/broker"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type Nats struct {
	config  *Config
	ctx     context.Context
	options *broker.Options
	conn    *nats.Conn

	subscriptions map[string]*nats.Subscription
	handlers      map[string]broker.Handler
}

func New(opts *broker.Options, cfg *Config) *Nats {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	brk := &Nats{
		config:        cfg,
		ctx:           context.Background(),
		options:       opts,
		subscriptions: make(map[string]*nats.Subscription),
		handlers:      make(map[string]broker.Handler),
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nats broker created",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	broker.Instance(opts.ID, brk)

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

func (brk *Nats) ID() uuid.UUID {
	return brk.options.ID
}

func (brk *Nats) Name() string {
	return brk.options.Name
}

func (brk *Nats) Connect() error {
	nc, err := nats.Connect(
		brk.config.URL,
	)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nats broker connect failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return err
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nats broker connected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"url", brk.config.URL,
	)

	brk.conn = nc

	// Handlers
	for topic, hdl := range brk.handlers {
		err := brk.Subscribe(topic, hdl)
		if err != nil {
			brk.options.Logger.ErrorContext(
				brk.ctx,
				"Nats broker subscribe failed",
				"broker", brk.String(),
				"id", brk.options.ID,
				"name", brk.options.Name,
				"topic", topic,
				"error", err.Error(),
			)
		}
	}

	return nil
}

func (brk *Nats) Disconnect() error {
	if brk.conn != nil && !brk.conn.IsClosed() {
		for topic := range brk.handlers {
			brk.Unsubscribe(topic)
		}

		brk.conn.Close()
		brk.conn = nil
		brk.options.Logger.InfoContext(
			brk.ctx,
			"Nats broker disconnected",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"url", brk.config.URL,
		)
	}

	return nil
}

func (brk *Nats) Publish(topic string, m *broker.Message) error {
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
		return errors.New("broker not connected")
	}

	msg := nats.NewMsg(topic)
	if m != nil {
		for k, v := range m.Metadata {
			msg.Header.Add(k, v)
		}

		m.Topic = topic
		msg.Data = m.Raw()
	}

	err := brk.conn.PublishMsg(msg)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nats broker publish failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return err
	}

	brk.options.Logger.DebugContext(
		brk.ctx,
		"Nats broker published",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"topic", topic,
	)

	return nil
}

func (brk *Nats) Subscribe(topic string, h broker.Handler) error {
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
		return errors.New("broker not connected")
	}

	if brk.subscriptions[topic] != nil {
		return errors.New("topic already subscribed")
	}

	sub, err := brk.conn.Subscribe(topic, func(msg *nats.Msg) {
		if h != nil {
			m := broker.NewMessage(msg.Data)
			err := h(m)
			if err != nil {
				brk.options.Logger.ErrorContext(
					brk.ctx,
					"Nats broker handler error",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
					"error", err.Error(),
				)
			} else {
				brk.options.Logger.DebugContext(
					brk.ctx,
					"Nats broker handler processed",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
				)
			}
		}
	})

	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nats broker subscribe failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return err
	}

	brk.options.Logger.DebugContext(
		brk.ctx,
		"Nats broker subscribed",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"topic", topic,
	)

	brk.subscriptions[topic] = sub

	return nil
}

func (brk *Nats) Unsubscribe(topic string) error {
	sub := brk.subscriptions[topic]
	if sub != nil {
		sub.Unsubscribe()
		delete(brk.subscriptions, topic)
	}

	return nil
}

func (brk *Nats) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		list := hdl.Register()
		maps.Copy(brk.handlers, list)
		brk.options.Logger.DebugContext(
			brk.ctx,
			"Nats handler registered",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"handler", hdl.Name(),
		)
	}
}

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	Register() map[string]broker.Handler
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

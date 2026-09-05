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
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/metrics"
)

var (
	// ErrBrokerNotConnected is a shared nats value.
	ErrBrokerNotConnected = errors.New("broker not connected")
	// ErrTopicAlreadySubscribed is a shared nats value.
	ErrTopicAlreadySubscribed = errors.New("topic already subscribed")
)

// Nats is a nats component.
type Nats struct {
	config  *Config
	ctx     context.Context
	options *broker.Options
	conn    *nats.Conn

	mu            sync.RWMutex
	subscriptions map[string]*nats.Subscription
	handlers      map[string]broker.Handler
}

// New creates a new instance (nil on invalid config).
func New(opts *broker.Options, cfg *Config) *Nats {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	if err := cfg.Validate(); err != nil {
		opts.Logger.ErrorContext(
			opts.Context,
			"Nats broker config invalid",
			"error", err.Error(),
		)

		return nil
	}

	brk := &Nats{
		config:        cfg,
		ctx:           opts.Context,
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

	broker.Set(brk)

	return brk
}

// Context returns the component context.
func (brk *Nats) Context() context.Context {
	return brk.ctx
}

// Options returns the runtime options.
func (brk *Nats) Options() *broker.Options {
	return brk.options
}

// String returns a human-readable name.
func (brk *Nats) String() string {
	return "nats"
}

// ID returns the unique instance ID.
func (brk *Nats) ID() uuid.UUID {
	return brk.options.ID
}

// Name returns the component name.
func (brk *Nats) Name() string {
	return brk.options.Name
}

// Connect connects to the backend.
func (brk *Nats) Connect() error {
	nc, err := nats.Connect(
		brk.config.URL,
	)
	metrics.BrokerConnected.WithLabelValues("nats").Set(0)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nats broker connect failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("nats broker connect (url %s): %w", infra.RedactDSN(brk.config.URL), err)
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nats broker connected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"url", infra.RedactDSN(brk.config.URL),
	)

	// Snapshot handlers under the lock; Disconnect must not race the
	// re-subscribe iteration.
	brk.mu.Lock()
	brk.conn = nc
	handlers := maps.Clone(brk.handlers)
	brk.mu.Unlock()
	metrics.BrokerConnected.WithLabelValues("nats").Set(1)

	// Handlers
	for topic, hdl := range handlers {
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

// Disconnect disconnects from the backend.
func (brk *Nats) Disconnect() error {
	var unsubErr error

	brk.mu.Lock()
	conn := brk.conn
	handlers := maps.Clone(brk.handlers)
	brk.mu.Unlock()

	if conn != nil && !conn.IsClosed() {
		for topic := range handlers {
			if err := brk.Unsubscribe(topic); err != nil {
				unsubErr = errors.Join(unsubErr, fmt.Errorf("nats broker unsubscribe (topic %s): %w", topic, err))
			}
		}

		conn.Close()
		brk.mu.Lock()
		brk.conn = nil
		brk.mu.Unlock()
		metrics.BrokerConnected.WithLabelValues("nats").Set(0)
		brk.options.Logger.InfoContext(
			brk.ctx,
			"Nats broker disconnected",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"url", infra.RedactDSN(brk.config.URL),
		)
	}

	return unsubErr
}

// Publish publishes a message.
func (brk *Nats) Publish(topic string, m *broker.Message) error {
	start := time.Now()
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
		metrics.ObserveBrokerPublish("nats", topic, start, ErrBrokerNotConnected)

		return ErrBrokerNotConnected
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
	metrics.ObserveBrokerPublish("nats", topic, start, err)
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

		return fmt.Errorf("nats broker publish (topic %s): %w", topic, err)
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

// Subscribe subscribes a handler.
func (brk *Nats) Subscribe(topic string, h broker.Handler) error {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
		metrics.BrokerSubscribeTotal.WithLabelValues("nats", topic, "error").Inc()

		return ErrBrokerNotConnected
	}

	// Check-and-insert under the same lock: two concurrent Subscribe calls
	// for one topic must not both pass the existence check.
	_, exists := brk.subscriptions[topic]
	if exists {
		metrics.BrokerSubscribeTotal.WithLabelValues("nats", topic, "dup").Inc()

		return ErrTopicAlreadySubscribed
	}

	sub, err := brk.conn.Subscribe(topic, func(msg *nats.Msg) {
		start := time.Now()
		result := "ok"
		defer func() {
			if r := recover(); r != nil {
				result = "panic"
				brk.options.Logger.ErrorContext(
					brk.ctx,
					"Nats broker handler panicked",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
					"panic", r,
				)
			}
			metrics.ObserveBrokerHandler("nats", topic, result, time.Since(start))
		}()
		if h != nil {
			m := broker.NewMessage(msg.Data)
			err := h(m)
			if err != nil {
				result = "error"
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
		metrics.BrokerSubscribeTotal.WithLabelValues("nats", topic, "error").Inc()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nats broker subscribe failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return fmt.Errorf("nats broker subscribe (topic %s): %w", topic, err)
	}

	metrics.BrokerSubscribeTotal.WithLabelValues("nats", topic, "ok").Inc()
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

// Unsubscribe removes a subscription.
func (brk *Nats) Unsubscribe(topic string) error {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	sub := brk.subscriptions[topic]
	if sub != nil {
		if err := sub.Unsubscribe(); err != nil {
			return fmt.Errorf("nats broker unsubscribe (topic %s): %w", topic, err)
		}

		delete(brk.subscriptions, topic)
	}

	return nil
}

// Handle registers handlers.
func (brk *Nats) Handle(hdls ...Handler) {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	for _, hdl := range hdls {
		if hdl == nil {
			continue
		}

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

/* {{{ [Handler]. */
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

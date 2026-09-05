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
 * @file jetstream.go
 * @package jetstream
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package jetstream

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/infra"
)

var (
	// ErrBrokerNotConnected is a shared jetstream value.
	ErrBrokerNotConnected = errors.New("broker not connected")
	// ErrTopicAlreadySubscribed is a shared jetstream value.
	ErrTopicAlreadySubscribed = errors.New("topic already subscribed")
)

// JetStream is a jetstream component.
type JetStream struct {
	config     *Config
	ctx        context.Context
	options    *broker.Options
	conn       *nats.Conn
	streamer   nats.JetStreamContext
	streamInfo *nats.StreamInfo

	mu            sync.RWMutex
	subscriptions map[string]*nats.Subscription
	handlers      map[string]broker.Handler
}

// New creates a new instance (nil on invalid config).
func New(opts *broker.Options, cfg *Config) *JetStream {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	if err := cfg.Validate(); err != nil {
		opts.Logger.ErrorContext(
			opts.Context,
			"Jetstream broker config invalid",
			"error", err.Error(),
		)

		return nil
	}

	brk := &JetStream{
		config:        cfg,
		ctx:           opts.Context,
		options:       opts,
		subscriptions: make(map[string]*nats.Subscription),
		handlers:      make(map[string]broker.Handler),
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Jetstream broker created",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	broker.Set(brk)

	return brk
}

// Context returns the component context.
func (brk *JetStream) Context() context.Context {
	return brk.ctx
}

// Options returns the runtime options.
func (brk *JetStream) Options() *broker.Options {
	return brk.options
}

// String returns a human-readable name.
func (brk *JetStream) String() string {
	return "jetstream"
}

// ID returns the unique instance ID.
func (brk *JetStream) ID() uuid.UUID {
	return brk.options.ID
}

// Name returns the component name.
func (brk *JetStream) Name() string {
	return brk.options.Name
}

// Connect connects to the backend.
func (brk *JetStream) Connect() error {
	nc, err := nats.Connect(
		brk.config.URL,
	)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Jetstream broker connect failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("jetstream broker connect (url %s): %w", infra.RedactDSN(brk.config.URL), err)
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Jetstream broker connected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"url", infra.RedactDSN(brk.config.URL),
	)

	jc, err := nc.JetStream()
	if err != nil {
		// The freshly dialed connection owns reconnect goroutines and a
		// socket: a failed stream context must not leak them.
		nc.Close()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Jetstream create stream context failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("jetstream broker create stream context: %w", err)
	}

	si, err := jc.AddStream(&nats.StreamConfig{
		Name:         brk.config.Stream.Name,
		Subjects:     brk.config.Stream.Subjects,
		MaxConsumers: brk.config.Stream.MaxConsumers,
	})
	if err != nil {
		nc.Close()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Jetstream create stream info failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("jetstream broker create stream (stream %s): %w", brk.config.Stream.Name, err)
	}

	brk.mu.Lock()
	brk.conn = nc
	brk.streamer = jc
	brk.streamInfo = si
	snapshot := maps.Clone(brk.handlers)
	brk.mu.Unlock()

	// Handlers
	for topic, hdl := range snapshot {
		err := brk.Subscribe(topic, hdl)
		if err != nil {
			brk.options.Logger.ErrorContext(
				brk.ctx,
				"Jetstream broker subscribe failed",
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
func (brk *JetStream) Disconnect() error {
	var unsubErr error

	brk.mu.Lock()
	conn := brk.conn
	topics := make([]string, 0, len(brk.handlers))
	for topic := range brk.handlers {
		topics = append(topics, topic)
	}

	brk.mu.Unlock()
	if conn != nil && !conn.IsClosed() {
		for _, topic := range topics {
			if err := brk.Unsubscribe(topic); err != nil {
				unsubErr = errors.Join(unsubErr, fmt.Errorf("jetstream broker unsubscribe (topic %s): %w", topic, err))
			}
		}

		conn.Close()
		brk.mu.Lock()
		brk.conn = nil
		brk.mu.Unlock()
		brk.options.Logger.InfoContext(
			brk.ctx,
			"Jetstream broker disconnected",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"url", infra.RedactDSN(brk.config.URL),
		)
	}

	return unsubErr
}

// Publish publishes a message.
func (brk *JetStream) Publish(topic string, m *broker.Message) error {
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
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

	ack, err := brk.streamer.PublishMsg(msg)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Jetstream broker publish failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return fmt.Errorf("jetstream broker publish (topic %s): %w", topic, err)
	}

	brk.options.Logger.DebugContext(
		brk.ctx,
		"Jetstream broker published",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"topic", topic,
		"ack", ack.Sequence,
	)

	return nil
}

// Subscribe subscribes a handler.
func (brk *JetStream) Subscribe(topic string, h broker.Handler) error {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	if brk.conn == nil || !brk.conn.IsConnected() || brk.conn.IsClosed() {
		return ErrBrokerNotConnected
	}

	// Check-and-insert under the same lock: two concurrent Subscribe calls
	// for one topic must not both pass the existence check.
	_, exists := brk.subscriptions[topic]
	if exists {
		return ErrTopicAlreadySubscribed
	}

	sub, err := brk.streamer.Subscribe(topic, func(msg *nats.Msg) {
		defer func() {
			if r := recover(); r != nil {
				brk.options.Logger.ErrorContext(
					brk.ctx,
					"Jetstream broker handler panicked",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
					"panic", r,
				)
				// A panicking handler must not silently consume the
				// message: NAK so it stays in-flight until AckWait.
				_ = msg.Nak()
			}
		}()
		if h != nil {
			m := broker.NewMessage(msg.Data)
			err := h(m)
			if err != nil {
				brk.options.Logger.ErrorContext(
					brk.ctx,
					"Jetstream broker handler error",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
					"error", err.Error(),
				)
				_ = msg.Nak()
			} else {
				brk.options.Logger.DebugContext(
					brk.ctx,
					"Jetstream broker handler processed",
					"broker", brk.String(),
					"id", brk.options.ID,
					"name", brk.options.Name,
					"topic", topic,
				)
				_ = msg.Ack()
			}
		} else {
			_ = msg.Ack()
		}
	}, nats.ManualAck())
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Jetstream broker subscribe failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return fmt.Errorf("jetstream broker subscribe (topic %s): %w", topic, err)
	}

	brk.options.Logger.DebugContext(
		brk.ctx,
		"Jetstream broker subscribed",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"topic", topic,
	)

	brk.subscriptions[topic] = sub

	return nil
}

// Unsubscribe removes a subscription.
func (brk *JetStream) Unsubscribe(topic string) error {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	sub := brk.subscriptions[topic]
	if sub != nil {
		if err := sub.Unsubscribe(); err != nil {
			return fmt.Errorf("jetstream broker unsubscribe (topic %s): %w", topic, err)
		}

		delete(brk.subscriptions, topic)
	}

	return nil
}

// Handle registers handlers.
func (brk *JetStream) Handle(hdls ...Handler) {
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

// Deprecated: use JetStream.
type Jetstream = JetStream

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

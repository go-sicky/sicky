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
 * @file nsq.go
 * @package nsq
 * @author Dr.NP <np@herewe.tech>
 * @since 08/14/2024
 */

package nsq

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/metrics"
)

var (
	// ErrBrokerNotConnected is a shared nsq value.
	ErrBrokerNotConnected = errors.New("broker not connected")
	// ErrNilMessage is a shared nsq value.
	ErrNilMessage = errors.New("nil message")
)

// NSQ is an nsq component.
type NSQ struct {
	config    *Config
	ctx       context.Context
	options   *broker.Options
	producer  *nsq.Producer
	nsqCfg    *nsq.Config
	nsqLogger *nsqLogger

	mu            sync.RWMutex
	subscriptions map[string]*nsq.Consumer
	handlers      map[string]broker.Handler
}

// New creates a new instance (nil on invalid config).
func New(opts *broker.Options, cfg *Config) *NSQ {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	if err := cfg.Validate(); err != nil {
		opts.Logger.ErrorContext(
			opts.Context,
			"Nsq broker config invalid",
			"error", err.Error(),
		)

		return nil
	}

	brk := &NSQ{
		config:        cfg,
		ctx:           opts.Context,
		options:       opts,
		subscriptions: make(map[string]*nsq.Consumer),
		handlers:      make(map[string]broker.Handler),
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nsq broker created",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	brk.nsqCfg = nsq.NewConfig()
	brk.nsqCfg.MaxInFlight = cfg.MaxInFlight
	brk.nsqCfg.MsgTimeout = time.Duration(cfg.MsgTimeout) * time.Second
	brk.nsqCfg.MaxAttempts = cfg.MaxAttempts
	brk.nsqCfg.Deflate = false
	brk.nsqCfg.Snappy = false
	switch strings.ToLower(cfg.Compression) {
	case "deflate":
		brk.nsqCfg.Deflate = true
	case "snappy":
		brk.nsqCfg.Snappy = true
	}

	brk.nsqLogger = newNsqLogger(brk.options.Logger)
	broker.Set(brk)

	return brk
}

// Context returns the component context.
func (brk *NSQ) Context() context.Context {
	return brk.ctx
}

// Options returns the runtime options.
func (brk *NSQ) Options() *broker.Options {
	return brk.options
}

// String returns a human-readable name.
func (brk *NSQ) String() string {
	return "nsq"
}

// ID returns the unique instance ID.
func (brk *NSQ) ID() uuid.UUID {
	return brk.options.ID
}

// Name returns the component name.
func (brk *NSQ) Name() string {
	return brk.options.Name
}

// Connect connects to the backend.
// safeEndpoint redacts the endpoint for logs/errors only when it embeds
// userinfo; a plain host:port stays visible for debugging.
func safeEndpoint(ep string) string {
	if strings.Contains(ep, "@") {
		return infra.RedactDSN(ep)
	}

	return ep
}

func (brk *NSQ) Connect() error {
	p, err := nsq.NewProducer(brk.config.Endpoint, brk.nsqCfg)
	if err != nil {
		metrics.BrokerConnected.WithLabelValues("nsq").Set(0)
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker create producer failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("nsq broker create producer (endpoint %s): %w", safeEndpoint(brk.config.Endpoint), err)
	}

	p.SetLogger(brk.nsqLogger, nsq.LogLevelWarning)
	err = p.Ping()
	if err != nil {
		metrics.BrokerConnected.WithLabelValues("nsq").Set(0)
		// The producer keeps its connection goroutine alive after a ping
		// failure; stop it before returning.
		p.Stop()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker producer ping failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("nsq broker producer ping (endpoint %s): %w", safeEndpoint(brk.config.Endpoint), err)
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nsq broker connected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"addr", safeEndpoint(brk.config.Endpoint),
	)

	brk.mu.Lock()
	brk.producer = p
	snapshot := maps.Clone(brk.handlers)
	brk.mu.Unlock()
	metrics.BrokerConnected.WithLabelValues("nsq").Set(1)

	// Handlers
	for topic, hdl := range snapshot {
		err := brk.Subscribe(topic, hdl)
		if err != nil {
			brk.options.Logger.ErrorContext(
				brk.ctx,
				"Nsq broker subscribe failed",
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
func (brk *NSQ) Disconnect() error {
	var unsubErr error

	brk.mu.Lock()
	subs := maps.Clone(brk.subscriptions)
	brk.mu.Unlock()
	for topic := range subs {
		if err := brk.Unsubscribe(topic); err != nil {
			unsubErr = errors.Join(unsubErr, fmt.Errorf("nsq broker unsubscribe (topic %s): %w", topic, err))
		}
	}

	brk.mu.Lock()
	p := brk.producer
	brk.producer = nil
	brk.mu.Unlock()
	if p != nil {
		p.Stop()
	}
	metrics.BrokerConnected.WithLabelValues("nsq").Set(0)

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nsq broker disconnected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	return unsubErr
}

// Publish publishes a message.
func (brk *NSQ) Publish(topic string, m *broker.Message) error {
	start := time.Now()
	if brk.producer == nil {
		metrics.ObserveBrokerPublish("nsq", topic, start, ErrBrokerNotConnected)

		return ErrBrokerNotConnected
	}

	if m == nil {
		metrics.ObserveBrokerPublish("nsq", topic, start, ErrNilMessage)

		return ErrNilMessage
	}

	m.Topic = topic
	err := brk.producer.Publish(topic, m.Raw())
	metrics.ObserveBrokerPublish("nsq", topic, start, err)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker publish failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"error", err.Error(),
		)

		return fmt.Errorf("nsq broker publish (topic %s): %w", topic, err)
	}

	return nil
}

// Subscribe subscribes a handler.
func (brk *NSQ) Subscribe(topic string, h broker.Handler) error {
	brk.mu.RLock()
	_, dup := brk.subscriptions[topic]
	brk.mu.RUnlock()
	if dup {
		metrics.BrokerSubscribeTotal.WithLabelValues("nsq", topic, "dup").Inc()
		brk.options.Logger.DebugContext(
			brk.ctx,
			"Nsq broker duplicated subscription",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
		)

		return nil
	}

	consumer, err := nsq.NewConsumer(topic, brk.config.Channel, brk.nsqCfg)
	if err != nil {
		metrics.BrokerSubscribeTotal.WithLabelValues("nsq", topic, "error").Inc()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker create consumer failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
			"error", err.Error(),
		)

		return fmt.Errorf("nsq broker create consumer (topic %s channel %s): %w", topic, brk.config.Channel, err)
	}

	consumer.SetLogger(brk.nsqLogger, nsq.LogLevelWarning)
	// Register handler before dialing so Connect() replay sees it.
	if h != nil {
		brk.mu.Lock()
		brk.handlers[topic] = h
		brk.mu.Unlock()
	}

	consumer.AddHandler(&nsqHandler{
		Topic:   topic,
		Channel: brk.config.Channel,
		Broker:  brk,
	})
	err = consumer.ConnectToNSQD(brk.config.Endpoint)
	if err != nil {
		metrics.BrokerSubscribeTotal.WithLabelValues("nsq", topic, "error").Inc()
		// The consumer already spawned connection goroutines: stop it so
		// the half-open dial loop does not leak.
		consumer.Stop()
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker consumer connection failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
			"error", err.Error(),
		)

		return fmt.Errorf("nsq broker consumer connect (topic %s channel %s endpoint %s): %w", topic, brk.config.Channel, safeEndpoint(brk.config.Endpoint), err)
	}

	brk.mu.Lock()
	brk.subscriptions[topic] = consumer
	brk.mu.Unlock()
	metrics.BrokerSubscribeTotal.WithLabelValues("nsq", topic, "ok").Inc()
	brk.options.Logger.DebugContext(
		brk.ctx,
		"Nsq broker subscribed",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"topic", topic,
		"channel", brk.config.Channel,
	)

	return nil
}

// Unsubscribe removes a subscription.
func (brk *NSQ) Unsubscribe(topic string) error {
	brk.mu.Lock()
	defer brk.mu.Unlock()
	consumer := brk.subscriptions[topic]
	if consumer != nil {
		consumer.Stop()
		delete(brk.subscriptions, topic)
		brk.options.Logger.DebugContext(
			brk.ctx,
			"Nsq broker unsubscribed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
		)
	}

	return nil
}

// Handle registers handlers.
func (brk *NSQ) Handle(hdls ...Handler) {
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
			"Nsq handler registered",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"handler", hdl.Name(),
		)
	}
}

/* {{{ [Handler]. */
type nsqHandler struct {
	Topic   string
	Channel string
	Broker  *NSQ
}

// HandleMessage is part of the public API.
func (h *nsqHandler) HandleMessage(m *nsq.Message) (err error) {
	start := time.Now()
	result := "ok"
	// A panicking handler must not ack the message (it would be
	// permanently lost): return an error so nsq requeues it.
	defer func() {
		if r := recover(); r != nil {
			result = "panic"
			h.Broker.options.Logger.ErrorContext(
				h.Broker.ctx,
				"Nsq broker handler panicked",
				"broker", h.Broker.String(),
				"id", h.Broker.options.ID,
				"name", h.Broker.options.Name,
				"topic", h.Topic,
				"channel", h.Channel,
				"panic", r,
			)
			err = fmt.Errorf("nsq broker handler panicked: %v", r)
		}
		if err != nil && result == "ok" {
			result = "requeued"
		}
		metrics.ObserveBrokerHandler("nsq", h.Topic, result, time.Since(start))
	}()
	h.Broker.options.Logger.DebugContext(
		h.Broker.ctx,
		"Nsq message received",
		"broker", h.Broker.String(),
		"id", h.Broker.options.ID,
		"name", h.Broker.options.Name,
		"topic", h.Topic,
		"channel", h.Channel,
	)

	h.Broker.mu.RLock()
	hdl := h.Broker.handlers[h.Topic]
	h.Broker.mu.RUnlock()
	if hdl != nil {
		msg := broker.NewMessage(m.Body)
		err := hdl(msg)
		if err != nil {
			h.Broker.options.Logger.ErrorContext(
				h.Broker.ctx,
				"Nsq broker handler failed",
				"broker", h.Broker.String(),
				"id", h.Broker.options.ID,
				"name", h.Broker.options.Name,
				"topic", h.Topic,
				"channel", h.Channel,
				"error", err.Error(),
			)

			return err
		}

		h.Broker.options.Logger.DebugContext(
			h.Broker.ctx,
			"Nsq broker handler processed",
			"broker", h.Broker.String(),
			"id", h.Broker.options.ID,
			"name", h.Broker.options.Name,
			"topic", h.Topic,
			"channel", h.Channel,
		)
	}

	return nil
}

// Handler is a nsq component.
type Handler interface {
	Name() string
	Type() string
	Register() map[string]broker.Handler
}

/* }}} */

// Deprecated: use NSQ.
type Nsq = NSQ

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

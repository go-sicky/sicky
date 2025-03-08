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
	"maps"
	"strings"
	"time"

	"github.com/go-sicky/sicky/broker"
	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"
)

type Nsq struct {
	config    *Config
	ctx       context.Context
	options   *broker.Options
	producer  *nsq.Producer
	nsqCfg    *nsq.Config
	nsqLogger *nsqLogger

	subscriptions map[string]*nsq.Consumer
	handlers      map[string]broker.Handler
}

func New(opts *broker.Options, cfg *Config) *Nsq {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	brk := &Nsq{
		config:        cfg,
		ctx:           context.Background(),
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
	p, err := nsq.NewProducer(brk.config.Endpoint, brk.nsqCfg)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker create producer failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return err
	}

	p.SetLogger(brk.nsqLogger, nsq.LogLevelWarning)
	err = p.Ping()
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker producer ping failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"error", err.Error(),
		)

		return err
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nsq broker connected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
		"addr", brk.config.Endpoint,
	)

	brk.producer = p

	// Handlers
	for topic, hdl := range brk.handlers {
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

func (brk *Nsq) Disconnect() error {
	for topic := range brk.subscriptions {
		brk.Unsubscribe(topic)
	}

	if brk.producer != nil {
		brk.producer.Stop()
		brk.producer = nil
	}

	brk.options.Logger.InfoContext(
		brk.ctx,
		"Nsq broker disconnected",
		"broker", brk.String(),
		"id", brk.options.ID,
		"name", brk.options.Name,
	)

	return nil
}

func (brk *Nsq) Publish(topic string, m *broker.Message) error {
	if brk.producer == nil {
		// No producer
		return nil
	}

	m.Topic = topic
	err := brk.producer.Publish(topic, m.Raw())
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

		return err
	}

	return nil
}

func (brk *Nsq) Subscribe(topic string, h broker.Handler) error {
	if brk.subscriptions[topic] != nil {
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

	consummer, err := nsq.NewConsumer(topic, brk.config.Channel, brk.nsqCfg)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker create consummer failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
			"error", err.Error(),
		)

		return err
	}

	consummer.SetLogger(brk.nsqLogger, nsq.LogLevelWarning)
	consummer.AddHandler(&nsqHandler{
		Topic:   topic,
		Channel: brk.config.Channel,
		Broker:  brk,
	})
	err = consummer.ConnectToNSQD(brk.config.Endpoint)
	if err != nil {
		brk.options.Logger.ErrorContext(
			brk.ctx,
			"Nsq broker consummer connection failed",
			"broker", brk.String(),
			"id", brk.options.ID,
			"name", brk.options.Name,
			"topic", topic,
			"channel", brk.config.Channel,
			"error", err.Error(),
		)

		return err
	}

	brk.subscriptions[topic] = consummer
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

func (brk *Nsq) Unsubscribe(topic string) error {
	consummer := brk.subscriptions[topic]
	if consummer != nil {
		consummer.Stop()
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

func (brk *Nsq) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
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

/* {{{ [Handler] */
type nsqHandler struct {
	Topic   string
	Channel string
	Broker  *Nsq
}

func (h *nsqHandler) HandleMessage(m *nsq.Message) error {
	h.Broker.options.Logger.DebugContext(
		h.Broker.ctx,
		"Nsq message received",
		"broker", h.Broker.String(),
		"id", h.Broker.options.ID,
		"name", h.Broker.options.Name,
		"topic", h.Topic,
		"channel", h.Channel,
	)

	hdl := h.Broker.handlers[h.Topic]
	if h.Broker.handlers[h.Topic] != nil {
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
		} else {
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
	}

	return nil
}

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

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
 * @file broker.go
 * @package broker
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package broker

import (
	"context"

	"github.com/google/uuid"
)

type Broker interface {
	// Get context
	Context() context.Context
	// Server options
	Options() *Options
	// Stringify
	String() string
	// Broker ID
	ID() uuid.UUID
	// Broker name
	Name() string
	// Connect to broker
	Connect() error
	// Disconnect from broker
	Disconnect() error
	// Publish topic
	Publish(topic string, m *Message) error
	// Subscriber topic
	Subscribe(topic string, h Handler) error
	// Unsubscribe topic
	Unsubscribe(topic string) error
}

type Handler func(*Message) error

var (
	brokers       = make(map[uuid.UUID]Broker, 0)
	defaultBroker Broker
)

func Instance(id uuid.UUID, brk ...Broker) Broker {
	if len(brk) > 0 {
		brokers[id] = brk[0]
		defaultBroker = brk[0]

		return brk[0]
	}

	return brokers[id]
}

func Publish(topic string, m *Message) error {
	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Publish(topic, m)
}

func Subscribe(topic string, h Handler) error {
	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Subscribe(topic, h)
}

func Unsubscribe(topic string) error {
	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Unsubscribe(topic)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

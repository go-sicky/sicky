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
	"maps"
	"sync"

	"github.com/google/uuid"
)

// Broker is a broker component.
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

// Handler is a broker component.
type Handler func(*Message) error

var (
	brokers       = make(map[uuid.UUID]Broker, 0)
	defaultBroker Broker
	brkMu         sync.RWMutex
)

// Set registers broker instances; the first one becomes the default.
func Set(brks ...Broker) {
	brkMu.Lock()
	defer brkMu.Unlock()

	for _, brk := range brks {
		brokers[brk.ID()] = brk
		if defaultBroker == nil {
			defaultBroker = brk
		}
	}
}

// Get looks up a broker instance by ID.
func Get(id uuid.UUID) Broker {
	brkMu.RLock()
	defer brkMu.RUnlock()

	return brokers[id]
}

// Default returns the default broker instance.
func Default() Broker {
	brkMu.RLock()
	defer brkMu.RUnlock()

	return defaultBroker
}

// Brokers returns a copy of the broker registry.
func Brokers() map[uuid.UUID]Broker {
	brkMu.RLock()
	defer brkMu.RUnlock()

	out := make(map[uuid.UUID]Broker, len(brokers))
	maps.Copy(out, brokers)

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	brkMu.Lock()
	defer brkMu.Unlock()

	brokers = make(map[uuid.UUID]Broker, 0)
	defaultBroker = nil
}

/* {{{ [Helpers]. */
func Publish(topic string, m *Message) error {
	brkMu.RLock()
	defer brkMu.RUnlock()

	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Publish(topic, m)
}

// Subscribe subscribes a handler.
func Subscribe(topic string, h Handler) error {
	brkMu.RLock()
	defer brkMu.RUnlock()

	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Subscribe(topic, h)
}

// Unsubscribe removes a subscription.
func Unsubscribe(topic string) error {
	brkMu.RLock()
	defer brkMu.RUnlock()

	if defaultBroker == nil {
		return nil
	}

	return defaultBroker.Unsubscribe(topic)
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

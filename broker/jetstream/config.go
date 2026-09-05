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
 * @file config.go
 * @package jetstream
 * @author Dr.NP <np@herewe.tech>
 * @since 03/06/2025
 */

package jetstream

import (
	"errors"

	"github.com/nats-io/nats.go"
)

// ErrJetStreamNegativeMaxConsumers aborts startup on a negative
// max_consumers. Zero fills the default in Ensure.
var ErrJetStreamNegativeMaxConsumers = errors.New("jetstream max_consumers is negative")

const (
	// DefaultStreamName is a jetstream constant.
	DefaultStreamName = "sicky"
	// DefaultStreamMaxConsumers is a jetstream constant.
	DefaultStreamMaxConsumers = 256
	// Deprecated: misspelled, use DefaultStreamMaxConsumers.
	// DefaultStreamMaxConsummers is a jetstream constant.
	DefaultStreamMaxConsummers = DefaultStreamMaxConsumers
)

// StreamConfig is a jetstream component.
type StreamConfig struct {
	Name         string   `json:"name"          mapstructure:"name"          yaml:"name"`
	Subjects     []string `json:"subjects"      mapstructure:"subjects"      yaml:"subjects"`
	MaxConsumers int      `json:"max_consumers" mapstructure:"max_consumers" yaml:"max_consumers"`
	// Deprecated: misspelled, use MaxConsumers. Kept for Go API compat;
	// if set, it seeds MaxConsumers when the latter is zero.
	MaxConsummers int `json:"-" mapstructure:"max_consummers_deprecated" yaml:"-"`
}

// Config is a jetstream component.
type Config struct {
	URL    string        `json:"url"    mapstructure:"url"    yaml:"url"`
	Stream *StreamConfig `json:"stream" mapstructure:"stream" yaml:"stream"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		URL: nats.DefaultURL,
		Stream: &StreamConfig{
			Name:         DefaultStreamName,
			Subjects:     []string{"*"},
			MaxConsumers: DefaultStreamMaxConsumers,
		},
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.URL == "" {
		c.URL = nats.DefaultURL
	}

	if c.Stream == nil {
		c.Stream = DefaultConfig().Stream
	}

	if c.Stream.Name == "" {
		c.Stream.Name = DefaultStreamName
	}

	if c.Stream.Subjects == nil {
		c.Stream.Subjects = []string{"*"}
	}

	// Migrate deprecated misspelled field.
	if c.Stream.MaxConsumers == 0 && c.Stream.MaxConsummers != 0 {
		c.Stream.MaxConsumers = c.Stream.MaxConsummers
	}

	c.Stream.MaxConsummers = c.Stream.MaxConsumers

	return c
}

// Validate rejects a negative max_consumers. Zero values are valid
// (Ensure fills defaults); nil is valid (disabled).
func (c *Config) Validate() error {
	if c == nil || c.Stream == nil {
		return nil
	}

	if c.Stream.MaxConsumers < 0 {
		return ErrJetStreamNegativeMaxConsumers
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

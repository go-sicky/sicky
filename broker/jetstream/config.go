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

import "github.com/nats-io/nats.go"

const (
	DefaultStreamName          = "sicky"
	DefaultStreamMaxConsummers = 256
)

type StreamConfig struct {
	Name          string   `json:"name" yaml:"name" mapstructure:"name"`
	Subjects      []string `json:"subjects" yaml:"subjects" mapstructure:"subjects"`
	MaxConsummers int      `json:"max_consumers" yaml:"max_consumers" mapstructure:"max_consumers"`
}

type Config struct {
	URL    string        `json:"url" yaml:"url" mapstructure:"url"`
	Stream *StreamConfig `json:"stream" yaml:"stream" mapstructure:"stream"`
}

func DefaultConfig() *Config {
	return &Config{
		URL: nats.DefaultURL,
		Stream: &StreamConfig{
			Name:          DefaultStreamName,
			Subjects:      []string{"*"},
			MaxConsummers: DefaultStreamMaxConsummers,
		},
	}
}

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

	if c.Stream.MaxConsummers < 0 {
		c.Stream.MaxConsummers = DefaultStreamMaxConsummers
	}

	return c
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

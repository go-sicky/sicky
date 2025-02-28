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
 * @package nsq
 * @author Dr.NP <np@herewe.tech>
 * @since 08/14/2024
 */

package nsq

const (
	DefaultEndpoint    = "127.0.0.1:4150"
	DefaultChannel     = "sicky"
	DefaultMaxInFlight = 10
	DefaultMsgTimeout  = 60
	DefaultMaxAttempts = 10
	DefaultCompression = "none"
)

type Config struct {
	Endpoint    string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Channel     string `json:"channel" yaml:"channel" mapstructure:"channel"`
	MaxInFlight int    `json:"max_in_flight" yaml:"max_in_flight" mapstructure:"max_in_flight"`
	MsgTimeout  int    `json:"msg_timeout" yaml:"msg_timeout" mapstructure:"msg_timeout"`
	MaxAttempts uint16 `json:"max_attempts" yaml:"max_attempts" mapstructure:"max_attempts"`
	Compression string `json:"compression" yaml:"compression" mapstructure:"compression"`
}

func DefaultConfig() *Config {
	return &Config{
		Endpoint:    DefaultEndpoint,
		Channel:     DefaultChannel,
		MaxInFlight: DefaultMaxInFlight,
		MsgTimeout:  DefaultMsgTimeout,
		MaxAttempts: DefaultMaxAttempts,
		Compression: DefaultCompression,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}

	if c.Channel == "" {
		c.Channel = DefaultChannel
	}

	if c.MaxInFlight == 0 {
		c.MaxInFlight = DefaultMaxInFlight
	}

	if c.MsgTimeout == 0 {
		c.MsgTimeout = DefaultMsgTimeout
	}

	if c.MaxAttempts == 0 {
		c.MaxAttempts = DefaultMaxAttempts
	}

	if c.Compression == "" {
		c.Compression = DefaultCompression
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

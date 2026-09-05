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

import (
	"errors"
	"strings"
)

var (
	// ErrNSQNegativeMaxInFlight aborts startup on a negative max_in_flight.
	ErrNSQNegativeMaxInFlight = errors.New("nsq max_in_flight is negative")
	// ErrNSQNegativeMsgTimeout aborts startup on a negative msg_timeout.
	ErrNSQNegativeMsgTimeout = errors.New("nsq msg_timeout is negative")
	// ErrNSQUnknownCompression aborts startup on an unknown compression.
	ErrNSQUnknownCompression = errors.New("nsq compression must be one of none, deflate, snappy")
)

const (
	// DefaultEndpoint is a nsq constant.
	DefaultEndpoint = "127.0.0.1:4150"
	// DefaultChannel is a nsq constant.
	DefaultChannel = "sicky"
	// DefaultMaxInFlight is a nsq constant.
	DefaultMaxInFlight = 10
	// DefaultMsgTimeout is a nsq constant.
	DefaultMsgTimeout = 60
	// DefaultMaxAttempts is a nsq constant.
	DefaultMaxAttempts = 10
	// DefaultCompression is a nsq constant.
	DefaultCompression = "none"
)

// Config is a nsq component.
type Config struct {
	Endpoint    string `json:"endpoint"      mapstructure:"endpoint"      yaml:"endpoint"`
	Channel     string `json:"channel"       mapstructure:"channel"       yaml:"channel"`
	MaxInFlight int    `json:"max_in_flight" mapstructure:"max_in_flight" yaml:"max_in_flight"`
	MsgTimeout  int    `json:"msg_timeout"   mapstructure:"msg_timeout"   yaml:"msg_timeout"`
	MaxAttempts uint16 `json:"max_attempts"  mapstructure:"max_attempts"  yaml:"max_attempts"`
	Compression string `json:"compression"   mapstructure:"compression"   yaml:"compression"`
}

// DefaultConfig returns the default configuration.
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

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
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

// Validate rejects negative timing/size values and unknown compression
// names. Zero values are valid (Ensure fills defaults).
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}

	if c.MaxInFlight < 0 {
		return ErrNSQNegativeMaxInFlight
	}

	if c.MsgTimeout < 0 {
		return ErrNSQNegativeMsgTimeout
	}

	switch strings.ToLower(c.Compression) {
	case "", "none", "deflate", "snappy":
	default:
		return ErrNSQUnknownCompression
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

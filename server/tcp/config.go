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
 * @package tcp
 * @author Dr.NP <np@herewe.tech>
 * @since 01/17/2025
 */

package tcp

const (
	// DefaultNetwork is a tcp constant.
	DefaultNetwork = "tcp"
	// DefaultAddress is a tcp constant.
	DefaultAddress = ":9981"
	// DefaultBufferSize is a tcp constant.
	DefaultBufferSize = 4096
	// DefaultMaxIdleDuration is a tcp constant.
	DefaultMaxIdleDuration = 60

	// MaxBufferSizeCap bounds per-connection BufferSize so a single
	// misconfiguration cannot OOM the process (1GB buffer x N conns).
	// MaxBufferSizeCap is a tcp constant.
	MaxBufferSizeCap = 1 << 20
	// MinReapIntervalSeconds floors the idle-session reaper tick.
	// MinReapIntervalSeconds is a tcp constant.
	MinReapIntervalSeconds = 5
)

// Config is a tcp component.
type Config struct {
	Network          string `json:"network"           mapstructure:"network"           yaml:"network"`
	Address          string `json:"address"           mapstructure:"address"           yaml:"address"`
	AdvertiseAddress string `json:"advertise_address" mapstructure:"advertise_address" yaml:"advertise_address"`
	BufferSize       int    `json:"buffer_size"       mapstructure:"buffer_size"       yaml:"buffer_size"`
	MaxIdleDuration  int    `json:"max_idle_duration" mapstructure:"max_idle_duration" yaml:"max_idle_duration"`
	// ReadTimeout is the per-read deadline in seconds. 0 disables it.
	ReadTimeout int `json:"read_timeout" mapstructure:"read_timeout" yaml:"read_timeout"`
	// WriteTimeout is the per-write deadline in seconds. 0 disables it.
	WriteTimeout int `json:"write_timeout" mapstructure:"write_timeout" yaml:"write_timeout"`
	// MaxSessions caps tracked sessions. 0 means unlimited.
	MaxSessions int `json:"max_sessions" mapstructure:"max_sessions" yaml:"max_sessions"`
	// MaxMessageBytes caps total bytes received per connection. 0 means
	// unlimited. Streaming/long-lived connections must set this high
	// enough or leave it 0 — exceeding it closes the connection.
	MaxMessageBytes int64 `json:"max_message_bytes" mapstructure:"max_message_bytes" yaml:"max_message_bytes"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Network:         DefaultNetwork,
		Address:         DefaultAddress,
		BufferSize:      DefaultBufferSize,
		MaxIdleDuration: DefaultMaxIdleDuration,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Network == "" {
		c.Network = DefaultNetwork
	}

	if c.Address == "" {
		c.Address = DefaultAddress
	}

	if c.BufferSize <= 0 {
		c.BufferSize = DefaultBufferSize
	}

	if c.BufferSize > MaxBufferSizeCap {
		c.BufferSize = MaxBufferSizeCap
	}

	if c.MaxIdleDuration <= 0 {
		c.MaxIdleDuration = DefaultMaxIdleDuration
	}

	if c.ReadTimeout < 0 {
		c.ReadTimeout = 0
	}

	if c.WriteTimeout < 0 {
		c.WriteTimeout = 0
	}

	if c.MaxSessions < 0 {
		c.MaxSessions = 0
	}

	if c.MaxMessageBytes < 0 {
		c.MaxMessageBytes = 0
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

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
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package websocket

import "errors"

const (
	// DefaultNetwork is a websocket constant.
	DefaultNetwork = "tcp"
	// DefaultAddress is a websocket constant.
	DefaultAddress = ":9991"
	// DefaultPath is a websocket constant.
	DefaultPath = "/conn"
	// DefaultPingDuration is a websocket constant.
	DefaultPingDuration = 5
	// DefaultMaxIdleDuration is a websocket constant.
	DefaultMaxIdleDuration = 60
	// DefaultShutdownTimeout is a websocket constant.
	DefaultShutdownTimeout = 10
	// DefaultTrustProxy enables Fiber's trusted-proxy handling unless
	// explicitly disabled via Config.TrustProxy.
	DefaultTrustProxy = true
)

// ErrIncompleteTLSConfig is returned when only one of tls_cert_pem and
// tls_key_pem is set. Serving plaintext with a half TLS configuration is
// never intended.
var ErrIncompleteTLSConfig = errors.New("websocket: tls_cert_pem and tls_key_pem must both be set or both empty")

// Config is a websocket component.
type Config struct {
	Network          string   `json:"network"           mapstructure:"network"           yaml:"network"`
	Address          string   `json:"address"           mapstructure:"address"           yaml:"address"`
	AdvertiseAddress string   `json:"advertise_address" mapstructure:"advertise_address" yaml:"advertise_address"`
	TLSCertPEM       string   `json:"tls_cert_pem"      mapstructure:"tls_cert_pem"      yaml:"tls_cert_pem"`
	TLSKeyPEM        string   `json:"tls_key_pem"       mapstructure:"tls_key_pem"       yaml:"tls_key_pem"`
	Path             string   `json:"path"              mapstructure:"path"              yaml:"path"`
	PingDuration     int      `json:"ping_duration"     mapstructure:"ping_duration"     yaml:"ping_duration"`
	MaxIdleDuration  int      `json:"max_idle_duration" mapstructure:"max_idle_duration" yaml:"max_idle_duration"`
	ShutdownTimeout  int      `json:"shutdown_timeout"  mapstructure:"shutdown_timeout"  yaml:"shutdown_timeout"`
	Origins          []string `json:"origins"           mapstructure:"origins"           yaml:"origins"`
	MaxMessageBytes  int      `json:"max_message_bytes" mapstructure:"max_message_bytes" yaml:"max_message_bytes"`
	// TrustProxy enables Fiber's trusted-proxy handling (X-Forwarded-*
	// honored for IP/host/scheme). It is a pointer so the zero config
	// keeps the secure default: nil means DefaultTrustProxy (true),
	// covering loopback, link-local and private ranges. Set an explicit
	// false only for direct-exposure deployments without a proxy.
	TrustProxy *bool `json:"trust_proxy" mapstructure:"trust_proxy" yaml:"trust_proxy"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Network:         DefaultNetwork,
		Address:         DefaultAddress,
		Path:            DefaultPath,
		PingDuration:    DefaultPingDuration,
		MaxIdleDuration: DefaultMaxIdleDuration,
		ShutdownTimeout: DefaultShutdownTimeout,
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

	if c.Path == "" {
		c.Path = DefaultPath
	}

	if c.PingDuration <= 0 {
		c.PingDuration = DefaultPingDuration
	}

	if c.MaxIdleDuration <= 0 {
		c.MaxIdleDuration = DefaultMaxIdleDuration
	}

	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

	// Origins: empty means same-origin enforcement (browser clients only),
	// non-browser clients without an Origin header are always allowed.
	// MaxMessageBytes: 0 means unlimited (read limit disabled).
	if c.TrustProxy == nil {
		c.TrustProxy = new(bool)
		*c.TrustProxy = DefaultTrustProxy
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

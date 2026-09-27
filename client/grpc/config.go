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
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package grpc

import (
	"errors"
	"strings"
	"time"

	"github.com/go-sicky/sicky/utils"
)

// ErrIncompleteTLSConfig is returned when only one half of the client
// certificate is configured.
var ErrIncompleteTLSConfig = errors.New("grpc client: tls_cert_pem and tls_key_pem must both be set or both empty")

// ErrInvalidTLSCA is returned when tls_ca_pem contains no usable
// certificate: trusting nothing would fail every connection anyway, and
// falling back to the system pool would silently change the trust
// decision.
var ErrInvalidTLSCA = errors.New("grpc client: tls_ca_pem contains no certificate")

const (
	// DefaultService is a grpc constant.
	DefaultService = "sicky"
	// DefaultNetwork is a grpc constant.
	DefaultNetwork = "tcp"
	// DefaultAddr is a grpc constant.
	DefaultAddr = ""
	// DefaultBalancer is a grpc constant.
	DefaultBalancer = "round_robin"
)

var balancers = map[string]bool{
	"least_request":        true,
	"pick_first":           true,
	"round_robin":          true,
	"weighted_round_robin": true,
}

type grpcServiceConfig struct {
	LoadBalancingConfig []map[string]map[string]any `json:"loadBalancingConfig,omitempty"`
	RetryPolicy         *struct {
		MaxAttempts    int    `json:"maxAttempts"`
		InitialBackoff string `json:"initialBackoff"`
		MaxBackoff     string `json:"maxBackoff"`
	} `json:"retryPolicy,omitempty"`
	HealthCheckConfig *struct {
		ServiceName        string `json:"serviceName"`
		FailureThreshold   int    `json:"failureThreshold"`
		UnhealthyThreshold int    `json:"unhealthyThreshold"`
		Interval           int    `json:"interval"`
	} `json:"healthCheckConfig,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

// Config is a grpc component.
type Config struct {
	Service    string `json:"service"      mapstructure:"service"      yaml:"service"`
	Network    string `json:"network"      mapstructure:"network"      yaml:"network"`
	Addr       string `json:"addr"         mapstructure:"addr"         yaml:"addr"`
	TLSCertPEM string `json:"tls_cert_pem" mapstructure:"tls_cert_pem" yaml:"tls_cert_pem"`
	TLSKeyPEM  string `json:"tls_key_pem"  mapstructure:"tls_key_pem"  yaml:"tls_key_pem"`
	// TLSCAPEM verifies the server certificate. Client certificates
	// (tls_cert_pem/tls_key_pem) stay optional: TLS and mutual TLS are
	// configured independently.
	TLSCAPEM string `json:"tls_ca_pem" mapstructure:"tls_ca_pem" yaml:"tls_ca_pem"`
	// TLSServerName overrides the SNI/verification name, for endpoints
	// dialed by IP or through a proxy.
	TLSServerName     string        `json:"tls_server_name"      mapstructure:"tls_server_name"      yaml:"tls_server_name"`
	ConnectionTimeout time.Duration `json:"connection_timeout"   mapstructure:"connection_timeout"   yaml:"connection_timeout"`
	MaxHeaderListSize uint32        `json:"max_header_list_size" mapstructure:"max_header_list_size" yaml:"max_header_list_size"`
	MaxMsgSize        int           `json:"max_msg_size"         mapstructure:"max_msg_size"         yaml:"max_msg_size"`
	ReadBufferSize    int           `json:"read_buffer_size"     mapstructure:"read_buffer_size"     yaml:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size"    mapstructure:"write_buffer_size"    yaml:"write_buffer_size"`
	Balancer          string        `json:"balancer"             mapstructure:"balancer"             yaml:"balancer"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Service:  DefaultService,
		Network:  DefaultNetwork,
		Addr:     DefaultAddr,
		Balancer: DefaultBalancer,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Service == "" {
		c.Service = DefaultService
	}

	// A bare number in a duration field (`read_timeout: 10`) decodes as
	// 10 nanoseconds and fails every request: read sub-millisecond
	// values as a count of seconds.
	c.ConnectionTimeout = utils.NormalizeDuration(c.ConnectionTimeout)

	if c.Network == "" {
		c.Network = DefaultNetwork
	}

	if c.Addr == "" {
		c.Addr = DefaultAddr
	}

	if c.Balancer == "" {
		c.Balancer = DefaultBalancer
	}

	vb := strings.ToLower(c.Balancer)
	if balancers[vb] {
		c.Balancer = vb
	}

	return c
}

// TLSEnabled reports whether the client will use TLS. Any TLS material
// turns it on - a CA or a server name alone verifies the server without
// presenting a client certificate, which is what most deployments need.
func (c *Config) TLSEnabled() bool {
	if c == nil {
		return false
	}

	return strings.TrimSpace(c.TLSCertPEM) != "" ||
		strings.TrimSpace(c.TLSKeyPEM) != "" ||
		strings.TrimSpace(c.TLSCAPEM) != "" ||
		strings.TrimSpace(c.TLSServerName) != ""
}

// Validate rejects half-TLS (BREAKING: previously silently downgraded to insecure).
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}

	certEmpty := strings.TrimSpace(c.TLSCertPEM) == ""
	keyEmpty := strings.TrimSpace(c.TLSKeyPEM) == ""
	if certEmpty != keyEmpty {
		return ErrIncompleteTLSConfig
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

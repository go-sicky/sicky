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
	"strings"
	"time"
)

const (
	DefaultService  = "sicky"
	DefaultNetwork  = "tcp"
	DefaultAddr     = "127.0.0.1:9991"
	DefaultBalancer = "round_robin"
)

var (
	balancers = map[string]bool{
		"least_request":        true,
		"pick_first":           true,
		"round_robin":          true,
		"weighted_round_robin": true,
	}
)

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

type Config struct {
	Service           string        `json:"service" yaml:"service" mapstructure:"service"`
	Network           string        `json:"network" yaml:"network" mapstructure:"network"`
	Addr              string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	TLSCertPEM        string        `json:"tls_cert_pem" yaml:"tls_cert_pem" mapstructure:"tls_cert_pem"`
	TLSKeyPEM         string        `json:"tls_key_pem" yaml:"tls_key_pem" mapstructure:"tls_key_pem"`
	ConnectionTimeout time.Duration `json:"connection_timeout" yaml:"connection_timeout" mapstructure:"connection_timeout"`
	MaxHeaderListSize uint32        `json:"max_header_list_size" yaml:"max_header_list_size" mapstructure:"max_header_list_size"`
	MaxMsgSize        int           `json:"max_msg_size" yaml:"max_msg_size" mapstructure:"max_msg_size"`
	ReadBufferSize    int           `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	Balancer          string        `json:"balancer" yaml:"balancer" mapstructure:"balancer"`
}

func DefaultConfig() *Config {
	return &Config{
		Network:  DefaultNetwork,
		Addr:     DefaultAddr,
		Balancer: DefaultBalancer,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Service == "" {
		c.Service = DefaultService
	}

	if c.Network == "" {
		c.Network = DefaultNetwork
	}

	if c.Addr == "" {
		c.Addr = DefaultAddr
	}

	vb := strings.ToLower(c.Balancer)
	if balancers[vb] {
		c.Balancer = vb
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

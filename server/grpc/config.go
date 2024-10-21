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

import "time"

const (
	DefaultNetwork = "tcp"
	DefaultAddr    = ":0"

	// AccessLogger
	DefaultRequestIDContextKey    = "requestid"
	DefaultTraceIDContextKey      = "traceid"
	DefaultSpanIDContextKey       = "spanid"
	DefaultParentSpanIDContextKey = "parentspanid"
	DefaultSampledContextKey      = "sampled"
	DefaultAccessLevel            = "debug"
	DefaultClientErrorLevel       = "warn"
	DefaultServerErrorLevel       = "error"
)

var (
	DefaultAccessLogger = &AccessLoggerConfig{
		RequestIDContextKey:    DefaultRequestIDContextKey,
		TraceIDContextKey:      DefaultTraceIDContextKey,
		SpanIDContextKey:       DefaultSpanIDContextKey,
		ParentSpanIDContextKey: DefaultParentSpanIDContextKey,
		SampledContextKey:      DefaultSampledContextKey,
		AccessLevel:            DefaultAccessLevel,
		ClientErrorLevel:       DefaultClientErrorLevel,
		ServerErrorLevel:       DefaultServerErrorLevel,
	}
)

type AccessLoggerConfig struct {
	RequestIDContextKey    string `json:"request_id_context_key" yaml:"request_id_context_key" mapstructure:"request_id_context_key"`
	TraceIDContextKey      string `json:"trace_id_context_key" yaml:"trace_id_context_key" mapstructure:"trace_id_context_key"`
	SpanIDContextKey       string `json:"span_id_context_key" yaml:"span_id_context_key" mapstructure:"span_id_context_key"`
	ParentSpanIDContextKey string `json:"parent_span_id_context_key" yaml:"parent_span_id_context_key" mapstructure:"parent_span_id_context_key"`
	SampledContextKey      string `json:"sampled_context_key" yaml:"sampled_context_key" mapstructure:"sampled_context_key"`
	AccessLevel            string `json:"access_level" yaml:"access_level" mapstructure:"access_level"`
	ClientErrorLevel       string `json:"client_error_level" yaml:"client_error_level" mapstructure:"client_error_level"`
	ServerErrorLevel       string `json:"server_error_level" yaml:"server_error_level" mapstructure:"server_error_level"`
}

type Config struct {
	Network              string              `json:"network" yaml:"network" mapstructure:"network"`
	Addr                 string              `json:"addr" yaml:"addr" mapstructure:"addr"`
	TLSCertPEM           string              `json:"tls_cert_pem" yaml:"tls_cert_pem" mapstructure:"tls_cert_pem"`
	TLSKeyPEM            string              `json:"tls_key_pem" yaml:"tls_key_pem" mapstructure:"tls_key_pem"`
	ConnectionTimeout    time.Duration       `json:"connection_timeout" yaml:"connection_timeout" mapstructure:"connection_timeout"`
	MaxConcurrentStreams uint32              `json:"max_concurrent_streams" yaml:"max_concurrent_streams" mapstructures:"max_concurrent_streams"`
	MaxHeaderListSize    uint32              `json:"max_header_list_size" yaml:"max_header_list_size" mapstructure:"max_header_list_size"`
	MaxRecvMsgSize       int                 `json:"max_recv_msg_size" yaml:"max_recv_msg_size" mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize       int                 `json:"max_send_msg_size" yaml:"max_send_msg_size" mapstructure:"max_send_msg_size"`
	ReadBufferSize       int                 `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize      int                 `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	AccessLogger         *AccessLoggerConfig `json:"access_logger" yaml:"access_logger" maptructure:"access_logger"`
}

func DefaultConfig() *Config {
	return &Config{
		Network: DefaultNetwork,
		Addr:    DefaultAddr,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Network == "" {
		c.Network = DefaultNetwork
	}

	if c.Addr == "" {
		c.Addr = DefaultAddr
	}

	if c.AccessLogger == nil {
		c.AccessLogger = &AccessLoggerConfig{}
	}

	if c.AccessLogger.RequestIDContextKey == "" {
		c.AccessLogger.RequestIDContextKey = DefaultRequestIDContextKey
	}

	if c.AccessLogger.TraceIDContextKey == "" {
		c.AccessLogger.TraceIDContextKey = DefaultTraceIDContextKey
	}

	if c.AccessLogger.TraceIDContextKey == "" {
		c.AccessLogger.TraceIDContextKey = DefaultTraceIDContextKey
	}

	if c.AccessLogger.SpanIDContextKey == "" {
		c.AccessLogger.SpanIDContextKey = DefaultSpanIDContextKey
	}

	if c.AccessLogger.ParentSpanIDContextKey == "" {
		c.AccessLogger.ParentSpanIDContextKey = DefaultParentSpanIDContextKey
	}

	if c.AccessLogger.SampledContextKey == "" {
		c.AccessLogger.SampledContextKey = DefaultSampledContextKey
	}

	if c.AccessLogger.AccessLevel == "" {
		c.AccessLogger.AccessLevel = DefaultAccessLevel
	}

	if c.AccessLogger.ClientErrorLevel == "" {
		c.AccessLogger.ClientErrorLevel = DefaultClientErrorLevel
	}

	if c.AccessLogger.ServerErrorLevel == "" {
		c.AccessLogger.ServerErrorLevel = DefaultServerErrorLevel
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

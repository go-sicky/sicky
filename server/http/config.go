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
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package http

const (
	DefaultNetwork = "tcp"
	DefaultAddress = ":9990"

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
	Network             string              `json:"network" yaml:"network" mapstructure:"network"`
	Address             string              `json:"address" yaml:"address" mapstructure:"address"`
	AdvertiseAddress    string              `json:"advertise_address" yaml:"advertise_address" mapstructure:"advertise_address"`
	TLSCertPEM          string              `json:"tls_cert_pem" yaml:"tls_cert_pem" mapstructure:"tls_cert_pem"`
	TLSKeyPEM           string              `json:"tls_key_pem" yaml:"tls_key_pem" mapstructure:"tls_key_pem"`
	StrictRouting       bool                `json:"strict_routing" yaml:"strict_routing" mapstructure:"strict_routing"`
	CaseSensitive       bool                `json:"case_sensitive" yaml:"case_sensitive" mapstructure:"case_sensitive"`
	Etag                bool                `json:"etag" yaml:"etag" mapstructure:"etag"`
	BodyLimit           int                 `json:"body_limit" yaml:"body_limit" mapstructure:"body_limit"`
	Concurrency         int                 `json:"concurrency" yaml:"concurrency" mapstructure:"concurrency"`
	ReadBufferSize      int                 `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize     int                 `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	DisableKeepAlive    bool                `json:"disable_keep_alive" yaml:"disable_keep_alive" mapstructure:"disable_keep_alive"`
	EnableSwagger       bool                `json:"enable_swagger" yaml:"enable_swagger" mapstructure:"enable_swagger"`
	SwaggerPageTitle    string              `json:"swagger_page_title" yaml:"swagger_page_title" mapstructure:"swagger_page_title"`
	SwaggerValidatorURL string              `json:"swagger_validator_url" yaml:"swagger_validator_url" mapstructure:"swagger_validator_url"`
	EnableStackTrace    bool                `json:"enable_stack_trace" yaml:"enable_trace_stack" mapstructure:"enable_stack_trace"`
	AccessLogger        *AccessLoggerConfig `json:"access_logger" yaml:"access_logger" maptructure:"access_logger"`
}

func DefaultConfig() *Config {
	return &Config{
		Network: DefaultNetwork,
		Address: DefaultAddress,
	}
}

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

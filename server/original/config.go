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
 * @package original
 * @author Dr.NP <np@herewe.tech>
 * @since 08/27/2025
 */

package original

const (
	DefaultNetwork = "tcp"
	DefaultAddress = ":9980"

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
	Network          string              `json:"network" yaml:"network" mapstructure:"network"`
	Address          string              `json:"address" yaml:"address" mapstructure:"address"`
	AdvertiseAddress string              `json:"advertise_address" yaml:"advertise_address"`
	TLSCertPEM       string              `json:"tls_cert_pem" yaml:"tls_cert_pem"`
	TLSKeyPem        string              `json:"tls_key_pem" yaml:"tls_key_pem"`
	BodyLimit        int                 `json:"body_limit" yaml:"body_limit"`
	DisableKeepAlive bool                `json:"disable_keep_alive" yaml:"disable_keep_alive"`
	EnableSwagger    bool                `json:"enable_swagger" yaml:"enable_swagger"`
	SwaggerPageTitle string              `json:"swagger_page_title" yaml:"swagger_page_title"`
	EnableStackTrace bool                `json:"enable_stack_trace" yaml:"enable_trace_stack"`
	AccessLogger     *AccessLoggerConfig `json:"access_logger" yaml:"access_logger"`
}

func DefaultConfig() *Config {
	return &Config{
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

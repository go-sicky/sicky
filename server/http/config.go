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
 * @since 08/27/2025
 */

package http

import "time"

const (
	// DefaultNetwork is a http constant.
	DefaultNetwork = "tcp"
	// DefaultAddress is a http constant.
	DefaultAddress = ":9980"

	// Default server timeouts mirror the manager server precedents.
	// DefaultReadTimeout is a http constant.
	DefaultReadTimeout = 10 * time.Second
	// DefaultReadHeaderTimeout is a http constant.
	DefaultReadHeaderTimeout = 10 * time.Second
	// DefaultWriteTimeout is a http constant.
	DefaultWriteTimeout = 10 * time.Second
	// DefaultIdleTimeout is a http constant.
	DefaultIdleTimeout = 60 * time.Second
	// DefaultMaxHeaderBytes is a http constant.
	DefaultMaxHeaderBytes = 1 << 20
	// DefaultBodyLimit is a http constant.
	DefaultBodyLimit = 4 << 20
	// DefaultShutdownTimeout is a http constant.
	DefaultShutdownTimeout = 10 * time.Second

	// AccessLogger
	// DefaultRequestIDContextKey is a http constant.
	DefaultRequestIDContextKey = "requestid"
	// DefaultTraceIDContextKey is a http constant.
	DefaultTraceIDContextKey = "traceid"
	// DefaultSpanIDContextKey is a http constant.
	DefaultSpanIDContextKey = "spanid"
	// DefaultParentSpanIDContextKey is a http constant.
	DefaultParentSpanIDContextKey = "parentspanid"
	// DefaultSampledContextKey is a http constant.
	DefaultSampledContextKey = "sampled"
	// B3 propagation header names (shared by metadata/propagation/tracer).
	// DefaultB3TraceIDHeader is a http constant.
	DefaultB3TraceIDHeader = "X-B3-Traceid"
	// DefaultB3SpanIDHeader is a http constant.
	DefaultB3SpanIDHeader = "X-B3-Spanid"
	// DefaultB3ParentSpanIDHeader is a http constant.
	DefaultB3ParentSpanIDHeader = "X-B3-Parentspanid"
	// DefaultB3SampledHeader is a http constant.
	DefaultB3SampledHeader = "X-B3-Sampled"
	// DefaultAccessLevel is a http constant.
	DefaultAccessLevel = "debug"
	// DefaultClientErrorLevel is a http constant.
	DefaultClientErrorLevel = "warn"
	// DefaultServerErrorLevel is a http constant.
	DefaultServerErrorLevel = "error"
)

// DefaultAccessLogger is a shared http value.
var DefaultAccessLogger = &AccessLoggerConfig{
	RequestIDContextKey:    DefaultRequestIDContextKey,
	TraceIDContextKey:      DefaultTraceIDContextKey,
	SpanIDContextKey:       DefaultSpanIDContextKey,
	ParentSpanIDContextKey: DefaultParentSpanIDContextKey,
	SampledContextKey:      DefaultSampledContextKey,
	AccessLevel:            DefaultAccessLevel,
	ClientErrorLevel:       DefaultClientErrorLevel,
	ServerErrorLevel:       DefaultServerErrorLevel,
}

// AccessLoggerConfig is a http component.
type AccessLoggerConfig struct {
	RequestIDContextKey    string `json:"request_id_context_key"     mapstructure:"request_id_context_key"     yaml:"request_id_context_key"`
	TraceIDContextKey      string `json:"trace_id_context_key"       mapstructure:"trace_id_context_key"       yaml:"trace_id_context_key"`
	SpanIDContextKey       string `json:"span_id_context_key"        mapstructure:"span_id_context_key"        yaml:"span_id_context_key"`
	ParentSpanIDContextKey string `json:"parent_span_id_context_key" mapstructure:"parent_span_id_context_key" yaml:"parent_span_id_context_key"`
	SampledContextKey      string `json:"sampled_context_key"        mapstructure:"sampled_context_key"        yaml:"sampled_context_key"`
	AccessLevel            string `json:"access_level"               mapstructure:"access_level"               yaml:"access_level"`
	ClientErrorLevel       string `json:"client_error_level"         mapstructure:"client_error_level"         yaml:"client_error_level"`
	ServerErrorLevel       string `json:"server_error_level"         mapstructure:"server_error_level"         yaml:"server_error_level"`
}

// Config is a http component.
type Config struct {
	Network          string              `json:"network"            mapstructure:"network"                yaml:"network"`
	Address          string              `json:"address"            mapstructure:"address"                yaml:"address"`
	AdvertiseAddress string              `json:"advertise_address"  mapstructure:"advertise_address"      yaml:"advertise_address"`
	TLSCertPEM       string              `json:"tls_cert_pem"       mapstructure:"tls_cert_pem"           yaml:"tls_cert_pem"`
	TLSKeyPEM        string              `json:"tls_key_pem"        mapstructure:"tls_key_pem"            yaml:"tls_key_pem"`
	TLSKeyPem        string              `json:"-"                  mapstructure:"tls_key_pem_deprecated" yaml:"-"`
	BodyLimit        int                 `json:"body_limit"         mapstructure:"body_limit"             yaml:"body_limit"`
	DisableKeepAlive bool                `json:"disable_keep_alive" mapstructure:"disable_keep_alive"     yaml:"disable_keep_alive"`
	EnableSwagger    bool                `json:"enable_swagger"     mapstructure:"enable_swagger"         yaml:"enable_swagger"`
	SwaggerPageTitle string              `json:"swagger_page_title" mapstructure:"swagger_page_title"     yaml:"swagger_page_title"`
	EnableStackTrace bool                `json:"enable_stack_trace" mapstructure:"enable_stack_trace"     yaml:"enable_stack_trace"`
	AccessLogger     *AccessLoggerConfig `json:"access_logger"      mapstructure:"access_logger"          yaml:"access_logger"`
	CORS             *CORSConfig         `json:"cors"               mapstructure:"cors"                   yaml:"cors"`
	// Timeouts guard against Slowloris / slow-read / slow-write DoS.
	// Zero values fall back to the defaults above; negatives are clamped.
	ReadTimeout       time.Duration `json:"read_timeout"        mapstructure:"read_timeout"        yaml:"read_timeout"`
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" mapstructure:"read_header_timeout" yaml:"read_header_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"       mapstructure:"write_timeout"       yaml:"write_timeout"`
	IdleTimeout       time.Duration `json:"idle_timeout"        mapstructure:"idle_timeout"        yaml:"idle_timeout"`
	MaxHeaderBytes    int           `json:"max_header_bytes"    mapstructure:"max_header_bytes"    yaml:"max_header_bytes"`
	// ShutdownTimeout bounds graceful shutdown; lingering connections
	// are cut off past the deadline instead of hanging Stop forever.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Address: DefaultAddress,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	// Migrate deprecated misspelled TLSKeyPem.
	if c.TLSKeyPEM == "" && c.TLSKeyPem != "" {
		c.TLSKeyPEM = c.TLSKeyPem
	}

	c.TLSKeyPem = c.TLSKeyPEM

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

	if c.CORS == nil {
		c.CORS = &CORSConfig{}
	}

	c.CORS.Ensure()

	if c.ReadTimeout < 0 {
		c.ReadTimeout = 0
	}

	if c.ReadTimeout == 0 {
		c.ReadTimeout = DefaultReadTimeout
	}

	if c.ReadHeaderTimeout < 0 {
		c.ReadHeaderTimeout = 0
	}

	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	if c.WriteTimeout < 0 {
		c.WriteTimeout = 0
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}

	if c.IdleTimeout < 0 {
		c.IdleTimeout = 0
	}

	if c.IdleTimeout == 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}

	if c.MaxHeaderBytes <= 0 {
		c.MaxHeaderBytes = DefaultMaxHeaderBytes
	}

	// Non-positive fills the 4MB default; this version offers no opt-out.
	// The middleware itself (NewBodyLimitMiddleware) treats non-positive
	// as disabled, but server config never passes such a value through.
	if c.BodyLimit < 0 {
		c.BodyLimit = 0
	}

	if c.BodyLimit == 0 {
		c.BodyLimit = DefaultBodyLimit
	}

	if c.ShutdownTimeout < 0 {
		c.ShutdownTimeout = 0
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
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

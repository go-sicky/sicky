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
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package fiber

import (
	"errors"
	"slices"
	"time"
)

const (
	// DefaultNetwork is a fiber constant.
	DefaultNetwork = "tcp"
	// DefaultAddress is a fiber constant.
	DefaultAddress = ":9990"

	// Default server timeouts mirror the manager server precedents.
	// DefaultReadTimeout is a fiber constant.
	DefaultReadTimeout = 10 * time.Second
	// DefaultWriteTimeout is a fiber constant.
	DefaultWriteTimeout = 10 * time.Second
	// DefaultIdleTimeout is a fiber constant.
	DefaultIdleTimeout = 60 * time.Second

	// DefaultBodyLimit is a fiber constant.
	DefaultBodyLimit = 4 << 20
	// DefaultConcurrency is a fiber constant.
	DefaultConcurrency = 256 * 1024
	// DefaultReadBufferSize is a fiber constant.
	DefaultReadBufferSize = 4096
	// DefaultWriteBufferSize is a fiber constant.
	DefaultWriteBufferSize = 4096

	// DefaultCORSMaxAge is the preflight cache lifetime in seconds.
	// DefaultCORSMaxAge is a fiber constant.
	DefaultCORSMaxAge = 86400

	// DefaultShutdownTimeout is a fiber constant.
	DefaultShutdownTimeout = 10 * time.Second

	// DefaultTrustProxy enables Fiber's trusted-proxy handling unless
	// explicitly disabled via Config.TrustProxy.
	DefaultTrustProxy = true

	// AccessLogger
	// DefaultRequestIDContextKey is a fiber constant.
	DefaultRequestIDContextKey = "requestid"
	// DefaultTraceIDContextKey is a fiber constant.
	DefaultTraceIDContextKey = "traceid"
	// DefaultSpanIDContextKey is a fiber constant.
	DefaultSpanIDContextKey = "spanid"
	// DefaultParentSpanIDContextKey is a fiber constant.
	DefaultParentSpanIDContextKey = "parentspanid"
	// DefaultSampledContextKey is a fiber constant.
	DefaultSampledContextKey = "sampled"
	// B3 propagation header names (shared by metadata/propagation/tracer).
	// DefaultB3TraceIDHeader is a fiber constant.
	DefaultB3TraceIDHeader = "X-B3-Traceid"
	// DefaultB3SpanIDHeader is a fiber constant.
	DefaultB3SpanIDHeader = "X-B3-Spanid"
	// DefaultB3ParentSpanIDHeader is a fiber constant.
	DefaultB3ParentSpanIDHeader = "X-B3-Parentspanid"
	// DefaultB3SampledHeader is a fiber constant.
	DefaultB3SampledHeader = "X-B3-Sampled"
	// DefaultAccessLevel is a fiber constant.
	DefaultAccessLevel = "debug"
	// DefaultClientErrorLevel is a fiber constant.
	DefaultClientErrorLevel = "warn"
	// DefaultServerErrorLevel is a fiber constant.
	DefaultServerErrorLevel = "error"
)

// DefaultAccessLogger is a shared fiber value.
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

// AccessLoggerConfig is a fiber component.
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

// Config is a fiber component.
type Config struct {
	Network             string              `json:"network"               mapstructure:"network"               yaml:"network"`
	Address             string              `json:"address"               mapstructure:"address"               yaml:"address"`
	AdvertiseAddress    string              `json:"advertise_address"     mapstructure:"advertise_address"     yaml:"advertise_address"`
	TLSCertPEM          string              `json:"tls_cert_pem"          mapstructure:"tls_cert_pem"          yaml:"tls_cert_pem"`
	TLSKeyPEM           string              `json:"tls_key_pem"           mapstructure:"tls_key_pem"           yaml:"tls_key_pem"`
	StrictRouting       bool                `json:"strict_routing"        mapstructure:"strict_routing"        yaml:"strict_routing"`
	CaseSensitive       bool                `json:"case_sensitive"        mapstructure:"case_sensitive"        yaml:"case_sensitive"`
	Etag                bool                `json:"etag"                  mapstructure:"etag"                  yaml:"etag"`
	BodyLimit           int                 `json:"body_limit"            mapstructure:"body_limit"            yaml:"body_limit"`
	Concurrency         int                 `json:"concurrency"           mapstructure:"concurrency"           yaml:"concurrency"`
	ReadBufferSize      int                 `json:"read_buffer_size"      mapstructure:"read_buffer_size"      yaml:"read_buffer_size"`
	WriteBufferSize     int                 `json:"write_buffer_size"     mapstructure:"write_buffer_size"     yaml:"write_buffer_size"`
	DisableKeepAlive    bool                `json:"disable_keep_alive"    mapstructure:"disable_keep_alive"    yaml:"disable_keep_alive"`
	EnableSwagger       bool                `json:"enable_swagger"        mapstructure:"enable_swagger"        yaml:"enable_swagger"`
	SwaggerPageTitle    string              `json:"swagger_page_title"    mapstructure:"swagger_page_title"    yaml:"swagger_page_title"`
	SwaggerValidatorURL string              `json:"swagger_validator_url" mapstructure:"swagger_validator_url" yaml:"swagger_validator_url"`
	EnableStackTrace    bool                `json:"enable_stack_trace"    mapstructure:"enable_stack_trace"    yaml:"enable_stack_trace"`
	AccessLogger        *AccessLoggerConfig `json:"access_logger"         mapstructure:"access_logger"         yaml:"access_logger"`
	CORS                *CORSConfig         `json:"cors"                  mapstructure:"cors"                  yaml:"cors"`
	// Timeouts guard against Slowloris / slow-read / slow-write DoS.
	// Zero values fall back to the defaults above; negatives are clamped.
	ReadTimeout  time.Duration `json:"read_timeout"  mapstructure:"read_timeout"  yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" mapstructure:"write_timeout" yaml:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"  mapstructure:"idle_timeout"  yaml:"idle_timeout"`
	// ShutdownTimeout bounds graceful shutdown; lingering connections
	// are cut off past the deadline instead of hanging Stop forever.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	// TrustProxy enables Fiber's trusted-proxy handling (X-Forwarded-*
	// honored for IP/host/scheme). It is a pointer so the zero config
	// keeps the secure default: nil means DefaultTrustProxy (true),
	// covering loopback, link-local and private ranges. Set an explicit
	// false only for direct-exposure deployments without a proxy.
	TrustProxy *bool `json:"trust_proxy" mapstructure:"trust_proxy" yaml:"trust_proxy"`
}

// CORSConfig whitelists cross-origin access. An empty AllowedOrigins
// disables the CORS middleware entirely (deny by default); use an
// explicit "*" entry only for public APIs, never with AllowCredentials.
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"   mapstructure:"allowed_origins"   yaml:"allowed_origins"`
	AllowCredentials bool     `json:"allow_credentials" mapstructure:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age"           mapstructure:"max_age"           yaml:"max_age"`
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *CORSConfig) Ensure() *CORSConfig {
	if c == nil {
		c = new(CORSConfig)
	}

	if c.MaxAge <= 0 {
		c.MaxAge = DefaultCORSMaxAge
	}

	return c
}

// Validate rejects wildcard-origin with credentials (browsers forbid it
// and it leaks authenticated responses to any site).
func (c *CORSConfig) Validate() error {
	if c == nil {
		return nil
	}

	if !c.AllowCredentials {
		return nil
	}

	if slices.Contains(c.AllowedOrigins, "*") {
		return errors.New("fiber: AllowedOrigins \"*\" cannot be combined with AllowCredentials")
	}

	return nil
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Network: DefaultNetwork,
		Address: DefaultAddress,
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

	// Non-positive fills the 4MB default; this version offers no opt-out.
	if c.BodyLimit < 0 {
		c.BodyLimit = 0
	}

	if c.BodyLimit == 0 {
		c.BodyLimit = DefaultBodyLimit
	}

	if c.Concurrency < 0 {
		c.Concurrency = 0
	}

	if c.Concurrency == 0 {
		c.Concurrency = DefaultConcurrency
	}

	if c.ReadBufferSize <= 0 {
		c.ReadBufferSize = DefaultReadBufferSize
	}

	if c.WriteBufferSize <= 0 {
		c.WriteBufferSize = DefaultWriteBufferSize
	}

	if c.ReadTimeout < 0 {
		c.ReadTimeout = 0
	}

	if c.ReadTimeout == 0 {
		c.ReadTimeout = DefaultReadTimeout
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

	if c.ShutdownTimeout < 0 {
		c.ShutdownTimeout = 0
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

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

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
	// DefaultNetwork is a grpc constant.
	DefaultNetwork = "tcp"
	// DefaultAddress is a grpc constant.
	DefaultAddress = ":0"

	// DefaultShutdownTimeout is a grpc constant.
	DefaultShutdownTimeout = 10 * time.Second

	// AccessLogger
	// DefaultRequestIDContextKey is a grpc constant.
	DefaultRequestIDContextKey = "requestid"
	// DefaultTraceIDContextKey is a grpc constant.
	DefaultTraceIDContextKey = "traceid"
	// DefaultSpanIDContextKey is a grpc constant.
	DefaultSpanIDContextKey = "spanid"
	// DefaultParentSpanIDContextKey is a grpc constant.
	DefaultParentSpanIDContextKey = "parentspanid"
	// DefaultSampledContextKey is a grpc constant.
	DefaultSampledContextKey = "sampled"
	// DefaultAccessLevel is a grpc constant.
	DefaultAccessLevel = "debug"
	// DefaultClientErrorLevel is a grpc constant.
	DefaultClientErrorLevel = "warn"
	// DefaultServerErrorLevel is a grpc constant.
	DefaultServerErrorLevel = "error"
)

// DefaultAccessLogger is a shared grpc value.
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

// AccessLoggerConfig is a grpc component.
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

// Config is a grpc component.
type Config struct {
	Network           string        `json:"network"            mapstructure:"network"            yaml:"network"`
	Address           string        `json:"address"            mapstructure:"address"            yaml:"address"`
	AdvertiseAddress  string        `json:"advertise_address"  mapstructure:"advertise_address"  yaml:"advertise_address"`
	TLSCertPEM        string        `json:"tls_cert_pem"       mapstructure:"tls_cert_pem"       yaml:"tls_cert_pem"`
	TLSKeyPEM         string        `json:"tls_key_pem"        mapstructure:"tls_key_pem"        yaml:"tls_key_pem"`
	ConnectionTimeout time.Duration `json:"connection_timeout" mapstructure:"connection_timeout" yaml:"connection_timeout"`
	// MaxConnectionAgeGrace is the drain grace after ConnectionTimeout
	// fires. Only meaningful together with ConnectionTimeout.
	MaxConnectionAgeGrace time.Duration `json:"max_connection_age_grace" mapstructure:"max_connection_age_grace" yaml:"max_connection_age_grace"`
	// Keepalive probes idle connections; all three must be positive to
	// take effect. Zero keeps the gRPC defaults.
	KeepaliveTime    time.Duration `json:"keepalive_time"    mapstructure:"keepalive_time"    yaml:"keepalive_time"`
	KeepaliveTimeout time.Duration `json:"keepalive_timeout" mapstructure:"keepalive_timeout" yaml:"keepalive_timeout"`
	// MaxConnectionIdle closes connections idle longer than this.
	// Zero keeps the gRPC default (never).
	MaxConnectionIdle time.Duration `json:"max_connection_idle" mapstructure:"max_connection_idle" yaml:"max_connection_idle"`
	// MinPingInterval is the minimum interval for client pings without
	// data (abuse guard; gRPC default is 5 minutes). Zero keeps default.
	MinPingInterval      time.Duration `json:"min_ping_interval"      mapstructure:"min_ping_interval"      yaml:"min_ping_interval"`
	MaxConcurrentStreams uint32        `json:"max_concurrent_streams" mapstructure:"max_concurrent_streams" yaml:"max_concurrent_streams"`
	MaxHeaderListSize    uint32        `json:"max_header_list_size"   mapstructure:"max_header_list_size"   yaml:"max_header_list_size"`
	MaxRecvMsgSize       int           `json:"max_recv_msg_size"      mapstructure:"max_recv_msg_size"      yaml:"max_recv_msg_size"`
	MaxSendMsgSize       int           `json:"max_send_msg_size"      mapstructure:"max_send_msg_size"      yaml:"max_send_msg_size"`
	ReadBufferSize       int           `json:"read_buffer_size"       mapstructure:"read_buffer_size"       yaml:"read_buffer_size"`
	WriteBufferSize      int           `json:"write_buffer_size"      mapstructure:"write_buffer_size"      yaml:"write_buffer_size"`
	// ShutdownTimeout bounds GracefulStop; past the deadline the server
	// is force-stopped instead of hanging Stop forever.
	ShutdownTimeout time.Duration `json:"shutdown_timeout" mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	// DisableReflection hides service descriptors (grpcurl). The zero
	// value keeps reflection enabled to preserve existing behavior.
	//
	// Deprecated: use EnableReflection instead. Reflection is now opt-in
	// (default off); DisableReflection is only honored for compatibility.
	DisableReflection bool `json:"disable_reflection" mapstructure:"disable_reflection" yaml:"disable_reflection"`
	// EnableReflection explicitly opts into grpc reflection. Default false.
	EnableReflection bool                `json:"enable_reflection" mapstructure:"enable_reflection" yaml:"enable_reflection"`
	AccessLogger     *AccessLoggerConfig `json:"access_logger"     mapstructure:"access_logger"     yaml:"access_logger"`
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

	// Clamp negative limits: they would otherwise pass straight into
	// grpc options with undefined behavior. Zero keeps the gRPC default.
	// NOTE: MaxSendMsgSize intentionally has no framework default — the
	// server only sends what local handlers produce (self-inflicted
	// direction), while the dangerous inbound direction is already
	// bounded by MaxRecvMsgSize (gRPC default 4MB).
	if c.MaxRecvMsgSize < 0 {
		c.MaxRecvMsgSize = 0
	}

	if c.MaxSendMsgSize < 0 {
		c.MaxSendMsgSize = 0
	}

	if c.ReadBufferSize < 0 {
		c.ReadBufferSize = 0
	}

	if c.WriteBufferSize < 0 {
		c.WriteBufferSize = 0
	}

	if c.ShutdownTimeout < 0 {
		c.ShutdownTimeout = 0
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

	for _, d := range []*time.Duration{
		&c.ConnectionTimeout, &c.MaxConnectionAgeGrace,
		&c.KeepaliveTime, &c.KeepaliveTimeout,
		&c.MaxConnectionIdle, &c.MinPingInterval,
	} {
		if *d < 0 {
			*d = 0
		}
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

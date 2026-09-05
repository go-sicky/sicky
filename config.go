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
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package sicky

import (
	"errors"
	"fmt"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/broker/jetstream"
	"github.com/go-sicky/sicky/broker/nats"
	"github.com/go-sicky/sicky/broker/nsq"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/registry/consul"
	"github.com/go-sicky/sicky/registry/local"
	"github.com/go-sicky/sicky/registry/redis"
)

const (
	// DefaultManagerAddress is a sicky constant.
	DefaultManagerAddress = ":8888"
	// DefaultMetricsPath is a sicky constant.
	DefaultMetricsPath = "/metrics"
	// DefaultHealthPath is a sicky constant.
	DefaultHealthPath = "/health"
	// DefaultLivePath is a sicky constant.
	DefaultLivePath = "/live"
	// DefaultReadyPath is a sicky constant.
	DefaultReadyPath = "/ready"
	// DefaultVersionPath is a sicky constant.
	DefaultVersionPath = "/version"
	// DefaultInfoPath is a sicky constant.
	DefaultInfoPath = "/info"
	// DefaultSwaggerPath is a sicky constant.
	DefaultSwaggerPath = "/swagger.json"
	// DefaultConfigPath is a sicky constant.
	DefaultConfigPath = "/config"
	// DefaultServicePoolPath is a sicky constant.
	DefaultServicePoolPath = "/services"
	// DefaultShutdownTimeout is a sicky constant.
	DefaultShutdownTimeout = 5 // seconds

	// DefaultManagerReadTimeout is a sicky constant.
	DefaultManagerReadTimeout = 10 // seconds
	// DefaultManagerWriteTimeout is a sicky constant.
	DefaultManagerWriteTimeout = 10 // seconds
	// DefaultManagerIdleTimeout is a sicky constant.
	DefaultManagerIdleTimeout = 60 // seconds
)

// ErrManagerIncompleteTLSConfig is returned when only one of
// ManagerConfig.TLSCertPEM/TLSKeyPEM is set.
var ErrManagerIncompleteTLSConfig = errors.New("manager: tls_cert_pem and tls_key_pem must both be set or both empty")

// ManagerConfig is a sicky component.
type ManagerConfig struct {
	Enable           bool   `json:"enable"            mapstructure:"enable"            yaml:"enable"`
	Address          string `json:"address"           mapstructure:"address"           yaml:"address"`
	AdvertiseAddress string `json:"advertise_address" mapstructure:"advertise_address" yaml:"advertise_address"`
	EnableSwagger    bool   `json:"enable_swagger"    mapstructure:"enable_swagger"    yaml:"enable_swagger"`
	ExposeConfig     bool   `json:"expose_config"     mapstructure:"expose_config"     yaml:"expose_config"`
	// AuthToken guards debug/topology endpoints (/config, /services)
	// with `Authorization: Bearer <token>`. Empty means no token auth, in
	// which case /config and /services only accept loopback clients.
	// /metrics stays public for Prometheus scraping.
	AuthToken string `json:"auth_token" mapstructure:"auth_token" yaml:"auth_token"`
	// HTTP server timeouts in seconds (Slowloris mitigation).
	ReadTimeout     int `json:"read_timeout"     mapstructure:"read_timeout"     yaml:"read_timeout"`
	WriteTimeout    int `json:"write_timeout"    mapstructure:"write_timeout"    yaml:"write_timeout"`
	IdleTimeout     int `json:"idle_timeout"     mapstructure:"idle_timeout"     yaml:"idle_timeout"`
	ShutdownTimeout int `json:"shutdown_timeout" mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	// Optional TLS for the manager listener (PEM-encoded). Both fields are
	// required together; a half-configured pair fails Start fast instead of
	// silently serving plaintext. Empty means plaintext (default).
	TLSCertPEM      string `json:"tls_cert_pem"      mapstructure:"tls_cert_pem"      yaml:"tls_cert_pem"`
	TLSKeyPEM       string `json:"tls_key_pem"       mapstructure:"tls_key_pem"       yaml:"tls_key_pem"`
	MetricsPath     string `json:"metrics_path"      mapstructure:"metrics_path"      yaml:"metrics_path"`
	HealthPath      string `json:"health_path"       mapstructure:"health_path"       yaml:"health_path"`
	LivePath        string `json:"live_path"         mapstructure:"live_path"         yaml:"live_path"`
	ReadyPath       string `json:"ready_path"        mapstructure:"ready_path"        yaml:"ready_path"`
	VersionPath     string `json:"version_path"      mapstructure:"version_path"      yaml:"version_path"`
	InfoPath        string `json:"info_path"         mapstructure:"info_path"         yaml:"info_path"`
	SwaggerPath     string `json:"swagger_path"      mapstructure:"swagger_path"      yaml:"swagger_path"`
	ConfigPath      string `json:"config_path"       mapstructure:"config_path"       yaml:"config_path"`
	ServicePoolPath string `json:"service_pool_path" mapstructure:"service_pool_path" yaml:"service_pool_path"`
}

// DefaultManagerConfig is part of the public API.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		Enable:          true,
		Address:         DefaultManagerAddress,
		MetricsPath:     DefaultMetricsPath,
		HealthPath:      DefaultHealthPath,
		LivePath:        DefaultLivePath,
		ReadyPath:       DefaultReadyPath,
		VersionPath:     DefaultVersionPath,
		InfoPath:        DefaultInfoPath,
		SwaggerPath:     DefaultSwaggerPath,
		ConfigPath:      DefaultConfigPath,
		ServicePoolPath: DefaultServicePoolPath,
		// ExposeConfig stays false by default: /config dumps secrets.
		ReadTimeout:     DefaultManagerReadTimeout,
		WriteTimeout:    DefaultManagerWriteTimeout,
		IdleTimeout:     DefaultManagerIdleTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *ManagerConfig) Ensure() *ManagerConfig {
	if c == nil {
		c = DefaultManagerConfig()
	}

	if c.Address == "" {
		c.Address = DefaultManagerAddress
	}

	if c.MetricsPath == "" {
		c.MetricsPath = DefaultMetricsPath
	}

	if c.HealthPath == "" {
		c.HealthPath = DefaultHealthPath
	}

	if c.LivePath == "" {
		c.LivePath = DefaultLivePath
	}

	if c.ReadyPath == "" {
		c.ReadyPath = DefaultReadyPath
	}

	if c.VersionPath == "" {
		c.VersionPath = DefaultVersionPath
	}

	if c.InfoPath == "" {
		c.InfoPath = DefaultInfoPath
	}

	if c.SwaggerPath == "" {
		c.SwaggerPath = DefaultSwaggerPath
	}

	if c.ConfigPath == "" {
		c.ConfigPath = DefaultConfigPath
	}

	if c.ServicePoolPath == "" {
		c.ServicePoolPath = DefaultServicePoolPath
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

	if c.ReadTimeout == 0 {
		c.ReadTimeout = DefaultManagerReadTimeout
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultManagerWriteTimeout
	}

	if c.IdleTimeout == 0 {
		c.IdleTimeout = DefaultManagerIdleTimeout
	}

	return c
}

// Validate rejects a half-configured TLS pair.
func (c *ManagerConfig) Validate() error {
	if c == nil {
		return nil
	}

	if (c.TLSCertPEM != "") != (c.TLSKeyPEM != "") {
		return ErrManagerIncompleteTLSConfig
	}

	return nil
}

// InfraConfig is a sicky component.
type InfraConfig struct {
	Badger     *infra.BadgerConfig     `json:"badger"     mapstructure:"badger"     yaml:"badger"`
	Bun        *infra.BunConfig        `json:"bun"        mapstructure:"bun"        yaml:"bun"`
	Clickhouse *infra.ClickHouseConfig `json:"clickhouse" mapstructure:"clickhouse" yaml:"clickhouse"`
	Elastic    *infra.ElasticConfig    `json:"elastic"    mapstructure:"elastic"    yaml:"elastic"`
	Mongo      *infra.MongoConfig      `json:"mongo"      mapstructure:"mongo"      yaml:"mongo"`
	MQTT       *infra.MQTTConfig       `json:"mqtt"       mapstructure:"mqtt"       yaml:"mqtt"`
	Nats       *infra.NATSConfig       `json:"nats"       mapstructure:"nats"       yaml:"nats"`
	Redis      *infra.RedisConfig      `json:"redis"      mapstructure:"redis"      yaml:"redis"`
	Ristretto  *infra.RistrettoConfig  `json:"ristretto"  mapstructure:"ristretto"  yaml:"ristretto"`
	S3         *infra.S3Config         `json:"s3"         mapstructure:"s3"         yaml:"s3"`
}

// TracerConfig is a sicky component.
type TracerConfig struct {
	Type           string            `json:"type"            mapstructure:"type"            yaml:"type"`
	DSN            string            `json:"dsn"             mapstructure:"dsn"             yaml:"dsn"`
	Endpoint       string            `json:"endpoint"        mapstructure:"endpoint"        yaml:"endpoint"`
	ServiceName    string            `json:"service_name"    mapstructure:"service_name"    yaml:"service_name"`
	ServiceVersion string            `json:"service_version" mapstructure:"service_version" yaml:"service_version"`
	Compress       bool              `json:"compress"        mapstructure:"compress"        yaml:"compress"`
	Timeout        int               `json:"timeout"         mapstructure:"timeout"         yaml:"timeout"`
	Insecure       bool              `json:"insecure"        mapstructure:"insecure"        yaml:"insecure"`
	Headers        map[string]string `json:"headers"         mapstructure:"headers"         yaml:"headers"`
	PrettyPrint    bool              `json:"pretty_print"    mapstructure:"pretty_print"    yaml:"pretty_print"`
	Timestamps     bool              `json:"timestamps"      mapstructure:"timestamps"      yaml:"timestamps"`
	SampleRate     float64           `json:"sample_rate"     mapstructure:"sample_rate"     yaml:"sample_rate"`
}

// Tracer validation sentinels (presence-means-enabled aborts like infra).
var (
	ErrTracerUnknownType = errors.New("unknown tracer type")
	ErrTracerNoDSN       = errors.New("uptrace tracer DSN is required")
	ErrTracerNoEndpoint  = errors.New("otlp tracer endpoint is required")
)

// Validate checks the tracer selection. Uptrace requires DSN (abort);
// OTLP grpc/http fall back to defaults with endpoint required after Ensure.
func (c *TracerConfig) Validate() error {
	if c == nil {
		return nil
	}

	switch c.Type {
	case "", DefaultTracerType, tracerTypeGRPC, tracerTypeHTTP, tracerTypeStdout, tracerTypeUptrace:
	default:
		return fmt.Errorf("%w: %s", ErrTracerUnknownType, c.Type)
	}

	if c.Type == tracerTypeUptrace && c.DSN == "" {
		return ErrTracerNoDSN
	}

	return nil
}

// Ensure fills tracer defaults: empty type -> "none", out-of-range
// sample rate -> 1.0. ServiceName/Version stay empty here so the
// orchestrator can fall back to AppName/Version at runtime.
func (c *TracerConfig) Ensure() *TracerConfig {
	if c == nil {
		c = &TracerConfig{Type: DefaultTracerType}
	}

	if c.Type == "" {
		c.Type = DefaultTracerType
	}

	if c.SampleRate < 0.0 || c.SampleRate > 1.0 {
		c.SampleRate = 1.0
	}

	return c
}

const (
	// DefaultLogLevel is a sicky constant.
	DefaultLogLevel = "info"
	// DefaultTracerType is a sicky constant.
	DefaultTracerType = "none"

	// Tracer backend selectors (unexported: config surface stays stringly
	// typed for viper/mapstructure compat).
	tracerTypeGRPC    = "grpc"
	tracerTypeHTTP    = "http"
	tracerTypeStdout  = "stdout"
	tracerTypeUptrace = "uptrace"
)

// Config is a sicky component.
type Config struct {
	LogLevel string         `json:"log_level" mapstructure:"log_level" yaml:"log_level"`
	Manager  *ManagerConfig `json:"manager"   mapstructure:"manager"   yaml:"manager"`
	Infra    *InfraConfig   `json:"infra"     mapstructure:"infra"     yaml:"infra"`
	Tracer   *TracerConfig  `json:"tracer"    mapstructure:"tracer"    yaml:"tracer"`
	Registry struct {
		// Squashed for viper/mapstructure only; encoding/json+yaml keep the
		// outer "registry" envelope (no inline support), which is intended.
		registry.Config `mapstructure:",squash"`

		Consul *consul.Config `json:"consul" mapstructure:"consul" yaml:"consul"`
		Redis  *redis.Config  `json:"redis"  mapstructure:"redis"  yaml:"redis"`
		Local  *local.Config  `json:"local"  mapstructure:"local"  yaml:"local"`
	} `json:"registry" yaml:"registry" mapstructure:"registry"`
	Broker struct {
		// Same squash note as Registry above.
		broker.Config `mapstructure:",squash"`

		Nats      *nats.Config      `json:"nats"      mapstructure:"nats"      yaml:"nats"`
		Nsq       *nsq.Config       `json:"nsq"       mapstructure:"nsq"       yaml:"nsq"`
		Jetstream *jetstream.Config `json:"jetstream" mapstructure:"jetstream" yaml:"jetstream"`
	} `json:"broker" yaml:"broker" mapstructure:"broker"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		LogLevel: DefaultLogLevel,
		Manager:  DefaultManagerConfig(),
		Infra:    &InfraConfig{},
		Tracer: &TracerConfig{
			Type: DefaultTracerType,
		},
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}

	// nil Manager means disabled (BREAKING: previously nil became enabled).
	// Only fill defaults when caller provided a non-nil Manager.
	if c.Manager != nil {
		c.Manager = c.Manager.Ensure()
		if c.Manager.AdvertiseAddress == "" {
			c.Manager.AdvertiseAddress = c.Manager.Address
		}
	}

	if c.Infra == nil {
		c.Infra = &InfraConfig{}
	}

	if c.Infra.Badger != nil {
		c.Infra.Badger.Ensure()
	}

	if c.Infra.Bun != nil {
		c.Infra.Bun.Ensure()
	}

	if c.Infra.Clickhouse != nil {
		c.Infra.Clickhouse.Ensure()
	}

	if c.Infra.Elastic != nil {
		c.Infra.Elastic.Ensure()
	}

	if c.Infra.Mongo != nil {
		c.Infra.Mongo.Ensure()
	}

	if c.Infra.MQTT != nil {
		c.Infra.MQTT.Ensure()
	}

	if c.Infra.Nats != nil {
		c.Infra.Nats.Ensure()
	}

	if c.Infra.Redis != nil {
		c.Infra.Redis.Ensure()
	}

	if c.Infra.Ristretto != nil {
		c.Infra.Ristretto.Ensure()
	}

	if c.Infra.S3 != nil {
		c.Infra.S3.Ensure()
	}

	if c.Tracer == nil {
		c.Tracer = &TracerConfig{
			Type: DefaultTracerType,
		}
	}

	c.Tracer.Ensure()

	c.Registry.Ensure()
	if c.Registry.Consul != nil {
		c.Registry.Consul.Ensure()
	}

	if c.Registry.Redis != nil {
		c.Registry.Redis.Ensure()
	}

	if c.Registry.Local != nil {
		c.Registry.Local.Ensure()
	}

	c.Broker.Ensure()
	if c.Broker.Nats != nil {
		c.Broker.Nats.Ensure()
	}

	if c.Broker.Nsq != nil {
		c.Broker.Nsq.Ensure()
	}

	if c.Broker.Jetstream != nil {
		c.Broker.Jetstream.Ensure()
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

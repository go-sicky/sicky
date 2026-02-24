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
	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/broker/jetstream"
	"github.com/go-sicky/sicky/broker/nats"
	"github.com/go-sicky/sicky/broker/nsq"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/registry/consul"
	"github.com/go-sicky/sicky/registry/redis"
)

const (
	DefaultManagerAddress = ":8888"
	DefaultMetricsPath    = "/metrics"
	DefaultHealthPath     = "/health"
	DefaultVersionPath    = "/version"
	DefaultInfoPath       = "/info"
	DefaultSwaggerPath    = "/swagger.json"
)

type ManagerConfig struct {
	Enable           bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
	Address          string `json:"address" yaml:"address" mapstructure:"address"`
	AdvertiseAddress string `json:"advertise_address" yaml:"advertise_address" mapstructure:"advertise_address"`
	EnableSwagger    bool   `json:"enable_swagger" yaml:"enable_swagger" mapstructure:"enable_swagger"`
	MetricsPath      string `json:"metrics_path" yaml:"metrics_path" mapstructure:"metrics_path"`
	HealthPath       string `json:"health_path" yaml:"health_path" mapstructure:"health_path"`
	VersionPath      string `json:"version_path" yaml:"version_path" mapstructure:"version_path"`
	InfoPath         string `json:"info_path" yaml:"info_path" mapstructure:"info_path"`
	SwaggerPath      string `json:"swagger_path" yaml:"swagger_path" mapstructure:"swagger_path"`
}

func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		Enable:      true,
		Address:     DefaultManagerAddress,
		MetricsPath: DefaultMetricsPath,
		HealthPath:  DefaultHealthPath,
		VersionPath: DefaultVersionPath,
		InfoPath:    DefaultInfoPath,
		SwaggerPath: DefaultSwaggerPath,
	}
}

type InfraConfig struct {
	Badger     *infra.BadgerConfig     `json:"badger" yaml:"badger" mapstructure:"badger"`
	Bun        *infra.BunConfig        `json:"bun" yaml:"bun" mapstructure:"bun"`
	Clickhouse *infra.ClickhouseConfig `json:"clickhouse" yaml:"clickhouse" mapstructure:"clickhouse"`
	Elastic    *infra.ElasticConfig    `json:"elastic" yaml:"elastic" mapstructure:"elastic"`
	Mongo      *infra.MongoConfig      `json:"mongo" yaml:"mongo" mapstructure:"mongo"`
	MQTT       *infra.MQTTConfig       `json:"mqtt" yaml:"mqtt" mapstructure:"mqtt"`
	Nats       *infra.NatsConfig       `json:"nats" yaml:"nats" mapstructure:"nats"`
	Redis      *infra.RedisConfig      `json:"redis" yaml:"redis" mapstructure:"redis"`
	Ristretto  *infra.RistrettoConfig  `json:"ristretto" yaml:"ristretto" mapstructure:"ristretto"`
	S3         *infra.S3Config         `json:"s3" yaml:"s3" mapstructure:"s3"`
}

type TracerConfig struct {
	Type        string  `json:"type" yaml:"type" mapstructure:"type"`
	Endpoint    string  `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Compress    bool    `json:"compress" yaml:"compress" mapstructure:"compress"`
	Timeout     int     `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	PrettyPrint bool    `json:"pretty_print" yaml:"pretty_print" mapstructure:"pretty_print"`
	Timestamps  bool    `json:"timestamps" yaml:"timestamps" mapstructure:"timestamps"`
	SampleRate  float64 `json:"sample_rate" yaml:"sample_rate" mapstructure:"sample_rate"`
}

const (
	DefaultLogLevel                  = "info"
	DefaultRegistryPoolPurgeInterval = 0
	DefaultTracerType                = "none"
)

type Config struct {
	LogLevel                  string         `json:"log_level" yaml:"log_level" mapstructure:"log_level"`
	RegistryPoolPurgeInterval int            `json:"registry_pool_purge_interval" yaml:"registry_pool_purge_interval" mapstructure:"registry_pool_purge_interval"`
	Manager                   *ManagerConfig `json:"manager" yaml:"manager" mapstructure:"manager"`
	Infra                     *InfraConfig   `json:"infra" yaml:"infra" mapstructure:"infra"`
	Tracer                    *TracerConfig  `json:"tracer" yaml:"tracer" mapstructure:"tracer"`
	Registry                  struct {
		registry.Config

		Consul *consul.Config `json:"consul" yaml:"consul" mapstructure:"consul"`
		Redis  *redis.Config  `json:"redis" yaml:"redis" mapstructure:"redis"`
	} `json:"registry" yaml:"registry" mapstructure:"registry"`
	Broker struct {
		broker.Config

		Nats      *nats.Config      `json:"nats" yaml:"nats" mapstructure:"nats"`
		Nsq       *nsq.Config       `json:"nsq" yaml:"nsq" mapstructure:"nsq"`
		Jetstream *jetstream.Config `json:"jetstream" yaml:"jetstream" mapstructure:"jetstream"`
	} `json:"broker" yaml:"broker" mapstructure:"broker"`
}

func DefaultConfig() *Config {
	return &Config{
		LogLevel:                  DefaultLogLevel,
		RegistryPoolPurgeInterval: DefaultRegistryPoolPurgeInterval,
		Manager:                   DefaultManagerConfig(),
		Infra:                     &InfraConfig{},
		Tracer: &TracerConfig{
			Type: DefaultTracerType,
		},
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Manager != nil {
		if c.Manager.Address == "" {
			c.Manager.Address = DefaultManagerAddress
		}

		if c.Manager.MetricsPath == "" {
			c.Manager.MetricsPath = DefaultMetricsPath
		}

		if c.Manager.HealthPath == "" {
			c.Manager.HealthPath = DefaultHealthPath
		}

		if c.Manager.VersionPath == "" {
			c.Manager.VersionPath = DefaultVersionPath
		}

		if c.Manager.InfoPath == "" {
			c.Manager.InfoPath = DefaultInfoPath
		}

		if c.Manager.SwaggerPath == "" {
			c.Manager.SwaggerPath = DefaultSwaggerPath
		}
	} else {
		c.Manager = DefaultManagerConfig()
	}

	if c.Infra == nil {
		c.Infra = &InfraConfig{}
	}

	if c.Tracer == nil {
		c.Tracer = &TracerConfig{
			Type: DefaultTracerType,
		}
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

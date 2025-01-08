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
 * @package runtime
 * @author Dr.NP <np@herewe.tech>
 * @since 09/18/2024
 */

package runtime

import (
	"net/url"
	"strings"

	"github.com/go-sicky/sicky/driver"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

const (
	DefaultLogLevel                  = "info"
	DefaultRegistryPoolPurgeInterval = 0
	DefaultTracerType                = "none"
)

type Config struct {
	LogLevel                  string `json:"log_level" yaml:"log_level" mapstructure:"log_level"`
	RegistryPoolPurgeInterval int    `json:"registry_pool_purge_interval" yaml:"registry_pool_purge_interval" mapstructure:"registry_pool_purge_interval"`
	Tracer                    struct {
		Type        string `json:"type" yaml:"type" mapstructure:"type"`
		Endpoint    string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
		Compress    bool   `json:"compress" yaml:"compress" mapstructure:"compress"`
		Timeout     int    `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
		PrettyPrint bool   `json:"pretty_print" yaml:"pretty_print" mapstructure:"pretty_print"`
		Timestamps  bool   `json:"timestamps" yaml:"timestamps" mapstructure:"timestamps"`
	} `json:"tracer" yaml:"tracer" mapstructure:"tracer"`
	Metrics *metrics.Config `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	Driver  struct {
		DB    *driver.DBConfig    `json:"db" yaml:"db" mapstructure:"db"`
		Redis *driver.RedisConfig `json:"redis" yaml:"redis" mapstructure:"redis"`
		Nats  *driver.NatsConfig  `json:"nats" yaml:"nats" mapstructure:"nats"`
	} `json:"driver" yaml:"driver" mapstructure:"driver"`
}

func DefaultConfig() *Config {
	c := &Config{
		LogLevel:                  DefaultLogLevel,
		RegistryPoolPurgeInterval: DefaultRegistryPoolPurgeInterval,
	}
	c.Tracer.Type = DefaultTracerType

	return c
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.LogLevel == "" {
		c.LogLevel = DefaultLogLevel
	}

	return c
}

func LoadConfig(raw any) error {
	cfg := viper.New()
	cfg.SetConfigType(configType)

	// Try config source
	u, err := url.Parse(configLoc)
	if err == nil && u != nil && u.Scheme != "" && u.Path != "" {
		// Remote config source
		remote := strings.ToLower(u.Scheme)
		err = cfg.AddRemoteProvider(remote, u.Host, u.Path)
		if err != nil {
			logger.Logger.Fatal("Add remote config source failed", "error", err.Error())
		}

		err = cfg.ReadRemoteConfig()
	} else {
		// Local file
		cfg.SetConfigName(configLoc)
		cfg.AddConfigPath("/etc")
		cfg.AddConfigPath("/etc/" + AppName)
		cfg.AddConfigPath("$HOME/." + AppName)
		cfg.AddConfigPath(".")

		err = cfg.ReadInConfig()
	}

	if err != nil {
		logger.Logger.Fatal("Read config failed", "error", err.Error())
	}

	logger.Logger.Info("Config read", "location", configLoc)

	// Read config from environment variables
	cfg.SetEnvPrefix(strings.ToUpper(AppName))
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	// Marshal
	if raw != nil {
		err = cfg.Unmarshal(raw)
	}

	return err
}

func WatchConfig() error {
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

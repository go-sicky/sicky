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
 * @package redis
 * @author Dr.NP <np@herewe.tech>
 * @since 12/26/2025
 */

package redis

const (
	DefaultAddr           = "localhost:6379"
	DefaultDB             = 0
	DefaultPoolSize       = 10
	DefaultPassword       = ""
	DefaultNotifyKey      = "sicky-registry-notify"
	DefaultInstancePrefix = "sicky-registry-instance"
)

type Config struct {
	Addr           string `json:"addr" yaml:"addr" mapstructure:"addr"`
	Password       string `json:"password" yaml:"password" mapstructure:"password"`
	DB             int    `json:"db" yaml:"db" mapstructure:"db"`
	PoolSize       int    `json:"pool_size" yaml:"pool_size" mapstructure:"pool_size"`
	NotifyKey      string `json:"notify_key" yaml:"notify_key" mapstructure:"notify_key"`
	InstancePrefix string `json:"instance_prefix" yaml:"instance_prefix" mapstructure:"instance_prefix"`
}

func DefaultConfig() *Config {
	return &Config{
		Addr:           DefaultAddr,
		Password:       DefaultPassword,
		DB:             DefaultDB,
		PoolSize:       DefaultPoolSize,
		NotifyKey:      DefaultNotifyKey,
		InstancePrefix: DefaultInstancePrefix,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = &Config{}
	}

	if c.Addr == "" {
		c.Addr = DefaultAddr
	}

	if c.DB == 0 {
		c.DB = DefaultDB
	}

	if c.PoolSize == 0 {
		c.PoolSize = DefaultPoolSize
	}

	if c.NotifyKey == "" {
		c.NotifyKey = DefaultNotifyKey
	}

	if c.InstancePrefix == "" {
		c.InstancePrefix = DefaultInstancePrefix
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

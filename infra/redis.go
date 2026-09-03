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
 * @file redis.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package infra

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string `json:"addr" yaml:"addr" mapstructure:"addr"`
	Username string `json:"username" yaml:"username" mapstructure:"username"`
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	DB       int    `json:"db" yaml:"db" mapstructure:"db"`
	// EnableTLS wraps the connection in TLS 1.2+. TLSSkipVerify disables
	// certificate verification: development only, never in production.
	EnableTLS       bool `json:"enable_tls" yaml:"enable_tls" mapstructure:"enable_tls"`
	TLSSkipVerify   bool `json:"tls_skip_verify" yaml:"tls_skip_verify" mapstructure:"tls_skip_verify"`
	DialTimeoutSec  int  `json:"dial_timeout_sec" yaml:"dial_timeout_sec" mapstructure:"dial_timeout_sec"`
	ReadTimeoutSec  int  `json:"read_timeout_sec" yaml:"read_timeout_sec" mapstructure:"read_timeout_sec"`
	WriteTimeoutSec int  `json:"write_timeout_sec" yaml:"write_timeout_sec" mapstructure:"write_timeout_sec"`
	PoolSize        int  `json:"pool_size" yaml:"pool_size" mapstructure:"pool_size"`
	MinIdleConns    int  `json:"min_idle_conns" yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
}

// ErrRedisAddrEmpty / ErrRedisDBNegative / ErrRedisTimeoutInvalid abort
// startup: a non-nil RedisConfig means "enable redis".
var (
	ErrRedisAddrEmpty       = errors.New("infra: redis addr is empty")
	ErrRedisDBNegative      = errors.New("infra: redis db is negative")
	ErrRedisTimeoutInvalid  = errors.New("infra: redis timeout/pool setting is negative")
	ErrRedisIdleExceedsPool = errors.New("infra: redis min_idle_conns exceeds pool_size")
)

var Redis *redis.Client

func InitRedis(cfg *RedisConfig) (*redis.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Redis config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  time.Duration(cfg.DialTimeoutSec) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutSec) * time.Second,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
	if cfg.EnableTLS {
		rdb.Options().TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
		if cfg.TLSSkipVerify {
			logger.Logger.Warn("Redis TLS certificate verification disabled; use only for testing")
		}
	}

	// Ping with a timeout instead of Background(): an unreachable Redis
	// must not hang startup indefinitely.
	pctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	err := rdb.Ping(pctx).Err()
	if err != nil {
		logger.Logger.Error(
			"Redis initialize failed",
			"addr", cfg.Addr,
			"db", cfg.DB,
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking its sockets
		// and background goroutines.
		if cerr := rdb.Close(); cerr != nil {
			logger.Logger.Error(
				"Redis close after failed ping failed",
				"error", cerr.Error(),
			)
		}

		return nil, err
	}

	logger.Logger.Info(
		"Redis initialized",
		"addr", cfg.Addr,
		"db", cfg.DB,
		"tls", cfg.EnableTLS,
	)

	mu.Lock()
	defer mu.Unlock()
	if Redis != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("Redis already initialized, closing duplicate connection")
		if cerr := rdb.Close(); cerr != nil {
			logger.Logger.Error(
				"Redis duplicate close failed",
				"error", cerr.Error(),
			)
		}

		return Redis, nil
	}
	Redis = rdb

	return rdb, nil
}

// Default Redis timeouts in seconds.
const (
	DefaultRedisDialTimeoutSec  = 5
	DefaultRedisReadTimeoutSec  = 3
	DefaultRedisWriteTimeoutSec = 3
)

func (c *RedisConfig) Ensure() *RedisConfig {
	if c == nil {
		c = new(RedisConfig)
	}

	// Zero fills defaults; negative stays negative so Validate aborts
	// instead of silently swallowing a misconfiguration.
	if c.DialTimeoutSec == 0 {
		c.DialTimeoutSec = DefaultRedisDialTimeoutSec
	}

	if c.ReadTimeoutSec == 0 {
		c.ReadTimeoutSec = DefaultRedisReadTimeoutSec
	}

	if c.WriteTimeoutSec == 0 {
		c.WriteTimeoutSec = DefaultRedisWriteTimeoutSec
	}

	return c
}

func (c *RedisConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Addr) == "" {
		return ErrRedisAddrEmpty
	}

	if c.DB < 0 {
		return ErrRedisDBNegative
	}

	if c.DialTimeoutSec < 0 || c.ReadTimeoutSec < 0 || c.WriteTimeoutSec < 0 ||
		c.PoolSize < 0 || c.MinIdleConns < 0 {
		return ErrRedisTimeoutInvalid
	}

	if c.PoolSize > 0 && c.MinIdleConns > c.PoolSize {
		return ErrRedisIdleExceedsPool
	}

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

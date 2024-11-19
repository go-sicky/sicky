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
 * @package driver
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package driver

import (
	"context"

	"github.com/go-sicky/sicky/logger"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr     string `json:"addr" yaml:"addr" mapstructure:"addr"`
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	DB       int    `json:"db" yaml:"db" mapstructure:"db"`
}

var Redis *redis.Client

func InitRedis(cfg *RedisConfig) (*redis.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	err := rdb.Ping(context.TODO()).Err()
	if err != nil {
		logger.Logger.Error(
			"Redis initialize failed",
			"error", err.Error(),
		)

		return nil, err
	}

	logger.Logger.Info(
		"Redis initialized",
		"addr", cfg.Addr,
		"db", cfg.DB,
	)

	Redis = rdb

	return rdb, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

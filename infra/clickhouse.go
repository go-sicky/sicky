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
 * @file clickhouse.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package infra

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/uptrace/go-clickhouse/ch"
)

type ClickhouseConfig struct {
	DSN string `json:"dsn" yaml:"dsn" mapstructure:"dsn"`
}

// ErrClickhouseDSNEmpty aborts startup: a non-nil ClickhouseConfig means
// "enable clickhouse", and an empty DSN would otherwise fail late after a
// 5s ping timeout.
var ErrClickhouseDSNEmpty = errors.New("infra: clickhouse dsn is empty")

var Clickhouse *ch.DB

func InitClickhouse(cfg *ClickhouseConfig) (*ch.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Clickhouse config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	db := ch.Connect(ch.WithDSN(cfg.DSN))

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		logger.Logger.Error(
			"Clickhouse ping failed",
			"dsn", redactDSN(cfg.DSN),
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking it.
		if cerr := db.Close(); cerr != nil {
			logger.Logger.Error(
				"Clickhouse close after failed ping failed",
				"error", cerr.Error(),
			)
		}

		return nil, err
	}

	logger.Logger.Info(
		"Clickhouse initialized",
		"dsn", redactDSN(cfg.DSN),
	)

	mu.Lock()
	defer mu.Unlock()
	if Clickhouse != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("Clickhouse already initialized, closing duplicate connection")
		if cerr := db.Close(); cerr != nil {
			logger.Logger.Error(
				"Clickhouse duplicate close failed",
				"error", cerr.Error(),
			)
		}

		return Clickhouse, nil
	}
	Clickhouse = db

	return db, nil
}

func (c *ClickhouseConfig) Ensure() *ClickhouseConfig {
	if c == nil {
		c = new(ClickhouseConfig)
	}

	return c
}

func (c *ClickhouseConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.DSN) == "" {
		return ErrClickhouseDSNEmpty
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

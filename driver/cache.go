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
 * @file cache.go
 * @package driver
 * @author Dr.NP <np@herewe.tech>
 * @since 03/08/2025
 */

package driver

import (
	"github.com/dgraph-io/ristretto/v2"
	"github.com/go-sicky/sicky/logger"
)

type CacheConfig struct {
	NumCounters int64 `json:"num_counters" yaml:"num_counters" mapstructure:"num_counters"`
	MaxCost     int64 `json:"max_cost" yaml:"max_cost" mapstructure:"max_cost"`
	BufferItems int64 `json:"buffer_items" yaml:"buffer_items" mapstructure:"buffer_items"`
}

var Cache *ristretto.Cache[string, any]

func InitCache(cfg *CacheConfig) (*ristretto.Cache[string, any], error) {
	if cfg == nil {
		return nil, nil
	}

	cache, err := ristretto.NewCache(
		&ristretto.Config[string, any]{
			NumCounters: cfg.NumCounters,
			MaxCost:     cfg.MaxCost,
			BufferItems: cfg.BufferItems,
		},
	)
	if err != nil {
		logger.Logger.Error(
			"Ristretto cache initialize failed",
			"error", err.Error(),
		)

		return nil, err
	}

	logger.Logger.Info(
		"Ristretto cache initialized",
		"num_counters", cfg.NumCounters,
		"max_cost", cfg.MaxCost,
		"buffer_items", cfg.BufferItems,
	)

	Cache = cache

	return cache, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

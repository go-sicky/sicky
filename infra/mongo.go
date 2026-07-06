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
 * @file mongo.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package infra

import (
	"context"
	"time"

	"github.com/go-sicky/sicky/logger"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoConfig struct {
	URI string `json:"uri" yaml:"uri" mapstructure:"uri"`
	DB  string `json:"db" yaml:"db" mapstructure:"db"`
}

var Mongo *mongo.Client

func InitMongo(cfg *MongoConfig) (*mongo.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI))
	if err != nil {
		logger.Logger.Error(
			"Mongo connect failed",
			"uri", cfg.URI,
			"error", err.Error(),
		)

		return nil, err
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Logger.Error(
			"Mongo ping failed",
			"uri", cfg.URI,
			"error", err.Error(),
		)

		return nil, err
	}

	logger.Logger.Info(
		"Mongo initialized",
		"uri", cfg.URI,
		"db", cfg.DB,
	)

	if Mongo == nil {
		Mongo = client
	}

	return client, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

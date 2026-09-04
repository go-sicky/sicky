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
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/go-sicky/sicky/logger"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoConfig struct {
	URI string `json:"uri" yaml:"uri" mapstructure:"uri"`
	DB  string `json:"db" yaml:"db" mapstructure:"db"`
	// MaxPoolSize caps driver connections. 0 keeps the driver default
	// (100).
	MaxPoolSize uint64 `json:"max_pool_size" yaml:"max_pool_size" mapstructure:"max_pool_size"`
	// ConnectTimeoutSec bounds initial dial/server selection. 0 keeps the
	// driver default (30s); negative aborts startup.
	ConnectTimeoutSec int `json:"connect_timeout_sec" yaml:"connect_timeout_sec" mapstructure:"connect_timeout_sec"`
}

// ErrMongoURIEmpty aborts startup: a non-nil MongoConfig means "enable
// mongo", and an empty URI would otherwise fail late after a 5s ping
// timeout.
var (
	ErrMongoURIEmpty      = errors.New("infra: mongo uri is empty")
	ErrMongoOptionInvalid = errors.New("infra: mongo pool/timeout option is negative")
)

var Mongo *mongo.Client

func InitMongo(cfg *MongoConfig) (*mongo.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Mongo config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	clientOpts := options.Client().ApplyURI(cfg.URI)
	if cfg.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.ConnectTimeoutSec > 0 {
		clientOpts.SetConnectTimeout(time.Duration(cfg.ConnectTimeoutSec) * time.Second)
	}
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		logger.Logger.Error(
			"Mongo connect failed",
			"uri", redactDSN(cfg.URI),
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
			"uri", redactDSN(cfg.URI),
			"error", err.Error(),
		)

		// Disconnect the handle opened above instead of leaking it.
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		if derr := client.Disconnect(dctx); derr != nil {
			logger.Logger.Error(
				"Mongo disconnect after failed ping failed",
				"error", derr.Error(),
			)
		}

		return nil, err
	}

	logger.Logger.Info(
		"Mongo initialized",
		"uri", redactDSN(cfg.URI),
		"db", cfg.DB,
	)

	mu.Lock()
	defer mu.Unlock()
	if Mongo != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("Mongo already initialized, closing duplicate connection")
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		if derr := client.Disconnect(dctx); derr != nil {
			logger.Logger.Error(
				"Mongo duplicate disconnect failed",
				"error", derr.Error(),
			)
		}

		return Mongo, nil
	}
	Mongo = client
	mongoDBName = effectiveMongoDB(cfg)

	return client, nil
}

func (c *MongoConfig) Ensure() *MongoConfig {
	if c == nil {
		c = new(MongoConfig)
	}

	return c
}

func (c *MongoConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.URI) == "" {
		return ErrMongoURIEmpty
	}

	if c.ConnectTimeoutSec < 0 {
		return ErrMongoOptionInvalid
	}

	return nil
}

// mongoDBName remembers the effective database for GetMongoDB: the
// configured DB wins, else the database embedded in the URI path.
// Guarded by mu.
var mongoDBName string

// GetMongoDB returns a database handle: the explicit name wins, then the
// effective configured name (MongoConfig.DB, else the URI path database).
// It returns nil when mongo is not initialized or no name resolves.
func GetMongoDB(name ...string) *mongo.Database {
	mu.RLock()
	client := Mongo
	configured := mongoDBName
	mu.RUnlock()

	if client == nil {
		return nil
	}

	if len(name) > 0 && strings.TrimSpace(name[0]) != "" {
		return client.Database(strings.TrimSpace(name[0]))
	}

	if configured != "" {
		return client.Database(configured)
	}

	return nil
}

// effectiveMongoDB resolves cfg.DB, falling back to the URI path database.
func effectiveMongoDB(cfg *MongoConfig) string {
	if cfg == nil {
		return ""
	}

	if db := strings.TrimSpace(cfg.DB); db != "" {
		return db
	}

	if u, err := url.Parse(strings.TrimSpace(cfg.URI)); err == nil {
		return strings.Trim(u.Path, "/")
	}

	return ""
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

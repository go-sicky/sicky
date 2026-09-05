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
 * @package redis
 * @author Dr.NP <np@herewe.tech>
 * @since 12/26/2025
 */

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
)

// Redis is a redis component.
type Redis struct {
	config  *Config
	ctx     context.Context
	cancel  context.CancelFunc
	options *registry.Options
	client  *redis.Client
}

// New creates a new instance (nil on invalid config).
func New(opts *registry.Options, cfg *Config) *Redis {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	ctx, cancel := context.WithCancel(opts.Context)
	rg := &Redis{
		config:  cfg,
		ctx:     ctx,
		cancel:  cancel,
		options: opts,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	// Bound the ping: a server that accepts TCP but never answers must not
	// hang startup forever.
	pingCtx, pingCancel := context.WithTimeout(rg.ctx, 5*time.Second)
	err := rdb.Ping(pingCtx).Err()
	pingCancel()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Registry connection failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		// Close the half-open client: its pool and background goroutines
		// must not leak.
		_ = rdb.Close()

		return nil
	}

	rg.client = rdb
	rg.options.Logger.InfoContext(
		rg.ctx,
		"Registry connected",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	registry.Set(rg)

	return rg
}

// Context returns the component context.
func (rg *Redis) Context() context.Context {
	return rg.ctx
}

// Options returns the runtime options.
func (rg *Redis) Options() *registry.Options {
	return rg.options
}

// String returns a human-readable name.
func (rg *Redis) String() string {
	return "redis"
}

// ID returns the unique instance ID.
func (rg *Redis) ID() uuid.UUID {
	return rg.options.ID
}

// Name returns the component name.
func (rg *Redis) Name() string {
	return rg.options.Name
}

// Register registers the collector.
func (rg *Redis) Register(ins *registry.Instance) (err error) {
	start := time.Now()
	defer func() { metrics.ObserveRegistryOp("redis", "register", start, err) }()

	data, merr := json.Marshal(ins)
	if merr != nil {
		// A marshal failure must fail the registration instead of storing
		// an empty/corrupt value that every later Load() cannot parse.
		return fmt.Errorf("redis registry marshal (instance %s): %w", ins.ID, merr)
	}

	_, err = rg.client.HSet(rg.ctx, rg.config.InstanceKey, ins.ID.String(), string(data)).Result()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Register instance failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"instance_id", ins.ID.String(),
			"error", err.Error(),
		)

		return err
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Instance registered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"manager_address", ins.ManagerAddress,
		"manager_port", ins.ManagerPort,
		"service_name", ins.ServiceName,
		"instance_id", ins.ID.String(),
	)

	_, err = rg.client.Publish(rg.ctx, rg.config.NotifyKey, ins.ID.String()).Result()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Publish register notification failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"instance_id", ins.ID.String(),
			"error", err.Error(),
		)

		return err
	}

	return nil
}

// Deregister removes the registration.
func (rg *Redis) Deregister(id uuid.UUID) (err error) {
	start := time.Now()
	defer func() { metrics.ObserveRegistryOp("redis", "deregister", start, err) }()

	_, err = rg.client.HDel(rg.ctx, rg.config.InstanceKey, id.String()).Result()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Deregister instance failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"instance_id", id.String(),
			"error", err.Error(),
		)

		return err
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Instance deregistered",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"instance_id", id.String(),
	)

	_, err = rg.client.Publish(rg.ctx, rg.config.NotifyKey, id.String()).Result()

	return err
}

// CheckInstance checks instance liveness.
func (rg *Redis) CheckInstance(id uuid.UUID) bool {
	exists, err := rg.client.HExists(rg.ctx, rg.config.InstanceKey, id.String()).Result()
	if err != nil {
		metrics.RegistryOpsTotal.WithLabelValues("redis", "check", "error").Inc()
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Check instance failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"instance_id", id.String(),
			"error", err.Error(),
		)

		return false
	}

	result := "ok"
	if !exists {
		result = "missing"
	}
	metrics.RegistryOpsTotal.WithLabelValues("redis", "check", result).Inc()

	return exists
}

// Load loads persisted state.
func (rg *Redis) Load() (instances []*registry.Instance, err error) {
	start := time.Now()
	defer func() {
		metrics.ObserveRegistryOp("redis", "load", start, err)
		if err == nil {
			metrics.RegistryInstances.WithLabelValues("redis").Set(float64(len(instances)))
		}
	}()

	var res map[string]string
	res, err = rg.client.HGetAll(rg.ctx, rg.config.InstanceKey).Result()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Load instances failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		return nil, err
	}

	for _, v := range res {
		var ins registry.Instance
		err = json.Unmarshal([]byte(v), &ins)
		if err != nil {
			rg.options.Logger.ErrorContext(
				rg.ctx,
				"Unmarshal instance failed",
				"registry", rg.String(),
				"id", rg.options.ID,
				"name", rg.options.Name,
				"error", err.Error(),
			)

			continue
		}

		instances = append(instances, &ins)
	}

	return instances, nil
}

// Watch watches for changes.
func (rg *Redis) Watch() error {
	pubsub := rg.client.Subscribe(rg.ctx, rg.config.NotifyKey)

	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-rg.ctx.Done():
				_ = pubsub.Close()

				return
			case <-ch:
				// Reload services list
				ins, err := rg.Load()
				if err != nil {
					metrics.RegistryWatchEventsTotal.WithLabelValues("redis", "error").Inc()
					rg.options.Logger.ErrorContext(
						rg.ctx,
						"Reload services list failed",
						"registry", rg.String(),
						"id", rg.options.ID,
						"name", rg.options.Name,
						"error", err.Error(),
					)
					continue
				}

				rg.options.Logger.InfoContext(
					rg.ctx,
					"Watcher triggered",
					"registry", rg.String(),
				)

				registry.PurgePool(ins)
				metrics.RegistryWatchEventsTotal.WithLabelValues("redis", "reload").Inc()
			}
		}
	}()

	return nil
}

// Stop stops the component and releases resources.
func (rg *Redis) Stop() error {
	if rg.cancel != nil {
		rg.cancel()
	}

	if rg.client != nil {
		if err := rg.client.Close(); err != nil {
			rg.options.Logger.ErrorContext(
				rg.ctx,
				"Redis client close failed",
				"registry", rg.String(),
				"id", rg.options.ID,
				"name", rg.options.Name,
				"error", err.Error(),
			)

			return err
		}

		rg.options.Logger.InfoContext(
			rg.ctx,
			"Redis registry stopped",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)
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

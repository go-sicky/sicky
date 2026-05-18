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

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
	client  *redis.Client
}

func New(opts *registry.Options, cfg *Config) *Redis {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &Redis{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	err := rdb.Ping(context.TODO()).Err()
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Registry connection failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

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

func (rg *Redis) Context() context.Context {
	return rg.ctx
}

func (rg *Redis) Options() *registry.Options {
	return rg.options
}

func (rg *Redis) String() string {
	return "redis"
}

func (rg *Redis) ID() uuid.UUID {
	return rg.options.ID
}

func (rg *Redis) Name() string {
	return rg.options.Name
}

func (rg *Redis) Register(ins *registry.Instance) error {
	_, err := rg.client.HSet(rg.ctx, rg.config.InstanceKey, ins.ID.String(), utils.JSONAnyString(ins)).Result()
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
		"service_name", ins.ServiceMame,
		"instance_id", ins.ID.String(),
	)

	_, err = rg.client.Publish(rg.ctx, rg.config.NotifyKey, ins.ID.String()).Result()

	return nil
}

func (rg *Redis) Deregister(id uuid.UUID) error {
	_, err := rg.client.HDel(rg.ctx, rg.config.InstanceKey, id.String()).Result()
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

func (rg *Redis) CheckInstance(id uuid.UUID) bool {
	return false
}

func (rg *Redis) Load() ([]*registry.Instance, error) {
	var instances []*registry.Instance
	res, err := rg.client.HGetAll(rg.ctx, rg.config.InstanceKey).Result()
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

func (rg *Redis) Watch() error {
	pubsub := rg.client.Subscribe(rg.ctx, rg.config.NotifyKey)

	go func() {
		ch := pubsub.Channel()
		for {
			select {
			case <-rg.ctx.Done():
				pubsub.Close()
				return
			case <-ch:
				// Reload services list
				ins, err := rg.Load()
				if err != nil {
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
			}
		}
	}()

	return nil
}

func (rg *Redis) Stop() error {
	if rg.ctx != nil {
		_, done := context.WithCancel(rg.ctx)
		done()
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

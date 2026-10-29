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
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// redisMetricsHook records per-command latency/error into sicky_infra_ops.
// It is attached per client so every business call is observed without
// touching call sites. label names the component series: "redis" for the
// shared singleton, "redis:<name>" for a named instance.
type redisMetricsHook struct {
	label string
}

// component returns the metric label, defaulting to the shared series when the
// hook was built without one.
func (h redisMetricsHook) component() string {
	if h.label == "" {
		return "redis"
	}

	return h.label
}

func (h redisMetricsHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		start := time.Now()
		conn, err := next(ctx, network, addr)
		metrics.ObserveInfraOp(h.component(), "dial", start, err)

		return conn, err
	}
}

func (h redisMetricsHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		// Normalize because go-redis takes the command name from the
		// first argument: the typed methods pass a compile-time constant,
		// but Do and NewCmd carry caller-supplied args, so an unmapped
		// name would mint a permanent series per distinct value.
		metrics.ObserveInfraOp(h.component(), metrics.NormalizeRedisCommand(cmd.Name()), start, err)

		return err
	}
}

func (h redisMetricsHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		metrics.ObserveInfraOp(h.component(), "pipeline", start, err)

		return err
	}
}

// RedisConfig is a infra component.
type RedisConfig struct {
	Addr     string `json:"addr"     mapstructure:"addr"     yaml:"addr"`
	Username string `json:"username" mapstructure:"username" yaml:"username"`
	Password string `json:"password" mapstructure:"password" yaml:"password"`
	DB       int    `json:"db"       mapstructure:"db"       yaml:"db"`
	// EnableTLS wraps the connection in TLS 1.2+. TLSSkipVerify disables
	// certificate verification: development only, never in production.
	EnableTLS       bool `json:"enable_tls"        mapstructure:"enable_tls"        yaml:"enable_tls"`
	TLSSkipVerify   bool `json:"tls_skip_verify"   mapstructure:"tls_skip_verify"   yaml:"tls_skip_verify"`
	DialTimeoutSec  int  `json:"dial_timeout_sec"  mapstructure:"dial_timeout_sec"  yaml:"dial_timeout_sec"`
	ReadTimeoutSec  int  `json:"read_timeout_sec"  mapstructure:"read_timeout_sec"  yaml:"read_timeout_sec"`
	WriteTimeoutSec int  `json:"write_timeout_sec" mapstructure:"write_timeout_sec" yaml:"write_timeout_sec"`
	PoolSize        int  `json:"pool_size"         mapstructure:"pool_size"         yaml:"pool_size"`
	MinIdleConns    int  `json:"min_idle_conns"    mapstructure:"min_idle_conns"    yaml:"min_idle_conns"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete Redis configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*RedisConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// ErrRedisAddrEmpty / ErrRedisDBNegative / ErrRedisTimeoutInvalid abort
// startup: a non-nil RedisConfig means "enable redis".
var (
	ErrRedisAddrEmpty       = errors.New("infra: redis addr is empty")
	ErrRedisDBNegative      = errors.New("infra: redis db is negative")
	ErrRedisTimeoutInvalid  = errors.New("infra: redis timeout/pool setting is negative")
	ErrRedisIdleExceedsPool = errors.New("infra: redis min_idle_conns exceeds pool_size")
)

// Redis is a shared infra value.
var Redis *redis.Client

// InitRedis is part of the public API.
func InitRedis(cfg *RedisConfig) (*redis.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	client, err := initRedis(cfg)
	metrics.CountInfraInit("redis", err)
	if err == nil && client == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("redis", false)
	}

	return client, err
}

func initRedis(cfg *RedisConfig) (*redis.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"redis config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init redis instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init redis default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"redis default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("redis no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"redis using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"redis no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Redis
	mu.RUnlock()

	var topLevel *redis.Client

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newRedisClient(cfg, "redis")
		if err != nil {
			logger.Logger.Error(
				"init redis failed",
				"addr", cfg.Addr,
				"db", cfg.DB,
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]*redis.Client, len(names))
	for _, name := range names {
		if _, ok := redisInstances.get(name); ok {
			logger.Logger.Warn("redis instance already initialized, keeping existing client", "instance", name)

			continue
		}

		client, berr := newRedisClient(instances[name], instanceLabel("redis", name, selection))
		if berr != nil {
			metrics.CountInfraInit("redis:"+name, berr)
			logger.Logger.Error(
				"init redis instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackRedisClients(topLevel, built)

			return nil, fmt.Errorf("redis instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("redis:"+name, nil)
		built[name] = client
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Redis == nil {
			Redis = topLevel
			redisDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("redis already initialized, closing duplicate connection")
			closeRedisClient(topLevel, "redis")
		}
	}

	for _, name := range names {
		client, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := redisInstances.putFirst(name, client); !stored {
			logger.Logger.Warn("redis instance already initialized, closing duplicate connection", "instance", name)
			closeRedisClient(client, "redis:"+name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"redis instance initialized",
			"instance", name,
			"addr", instances[name].Addr,
			"db", instances[name].DB,
			"tls", instances[name].EnableTLS,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if client, ok := redisInstances.get(selection.Instance); ok && client != nil {
			mu.Lock()
			if Redis == nil {
				Redis = client
				redisDefaultInstance = selection.Instance
			}

			mu.Unlock()
		}
	}

	mu.RLock()
	client := Redis
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"redis initialized",
			"addr", cfg.Addr,
			"db", cfg.DB,
			"tls", cfg.EnableTLS,
		)
	}

	return client, nil
}

// newRedisClient opens, configures and pings one client. It touches no
// package-level state, so a build that fails leaves nothing registered.
func newRedisClient(cfg *RedisConfig, label string) (*redis.Client, error) {
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
	rdb.AddHook(redisMetricsHook{label: label})
	if cfg.EnableTLS {
		rdb.Options().TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			//nolint:gosec // G402: explicit opt-in TLSSkipVerify flag for test envs; Warn logged below
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}

		if cfg.TLSSkipVerify {
			logger.Logger.Warn(
				"redis TLS certificate verification disabled; use only for testing",
				"instance", label,
			)
		}
	}

	// Ping with a timeout instead of Background(): an unreachable Redis
	// must not hang startup indefinitely.
	pctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	err := rdb.Ping(pctx).Err()
	if err != nil {
		logger.Logger.Error(
			"redis initialize failed",
			"instance", label,
			"addr", cfg.Addr,
			"db", cfg.DB,
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking its sockets
		// and background goroutines.
		closeRedisClient(rdb, label)

		return nil, err
	}

	return rdb, nil
}

// closeRedisClient closes one client and logs a failure instead of returning
// it: the caller is already on an error or duplicate path.
func closeRedisClient(client *redis.Client, label string) {
	if client == nil {
		return
	}

	if cerr := client.Close(); cerr != nil {
		logger.Logger.Error(
			"redis close failed",
			"instance", label,
			"error", cerr.Error(),
		)
	}
}

// rollbackRedisClients closes everything one init call built but will not
// register, so a failed build leaves no pools, sockets or "up" series behind.
func rollbackRedisClients(defaultClient *redis.Client, built map[string]*redis.Client) {
	closeRedisClient(defaultClient, "redis")

	for name, client := range built {
		closeRedisClient(client, "redis:"+name)
		metrics.SetInfraUp("redis:"+name, false)
	}
}

// Default Redis timeouts in seconds.
const (
	DefaultRedisDialTimeoutSec  = 5
	DefaultRedisReadTimeoutSec  = 3
	DefaultRedisWriteTimeoutSec = 3
)

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
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

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *RedisConfig) Validate() error {
	if c == nil {
		return nil
	}

	if err := c.validateShape(); err != nil {
		return err
	}

	_, names, err := c.resolveInstances()
	if err != nil {
		return err
	}

	identityErr := c.validateIdentity()
	if _, err := resolveDefaultSelection(c.NoDefault, c.DefaultInstance, names, identityErr == nil, identityErr); err != nil {
		return err
	}

	return nil
}

// validateShape checks every value that does not depend on the connection
// identity, so an instance can be rejected before anything dials.
func (c *RedisConfig) validateShape() error {
	if c == nil {
		return nil
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

// validateIdentity requires the field that addresses the server.
func (c *RedisConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Addr) == "" {
		return ErrRedisAddrEmpty
	}

	return nil
}

func (c *RedisConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *RedisConfig) resolveInstances() (resolved map[string]*RedisConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*RedisConfig, len(names))
	for _, name := range names {
		raw := normalized[name]
		if raw.Instances != nil || raw.DefaultInstance != "" || raw.NoDefault {
			return nil, nil, fmt.Errorf("%w: instances.%s", ErrInstanceNested, name)
		}

		instance := *raw
		instance.Instances = nil
		instance.DefaultInstance = ""
		instance.NoDefault = false
		instance.Ensure()

		if err := instance.validateBase(); err != nil {
			return nil, nil, fmt.Errorf("instances.%s: %w", name, err)
		}

		resolved[name] = &instance
	}

	return resolved, names, nil
}

// ShutdownRedis closes the shared client and every named instance, then
// clears their references. Named instances are detached before closing so a
// reader cannot observe a closed handle through the registry; a default alias
// is closed exactly once.
//
//nolint:dupl // each component owns its globals; the detach/close/metrics shape is deliberately uniform
func ShutdownRedis() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := redisInstances.clear()

	mu.Lock()
	alias := redisDefaultInstance
	defaultClient := Redis
	Redis = nil
	redisDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultClient == nil {
		return nil
	}

	var errs []error

	for name, client := range instances {
		if client == nil {
			continue
		}

		if cerr := client.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("redis instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("redis:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultClient != nil {
		if cerr := defaultClient.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("redis: %w", cerr))
		}
	}

	metrics.SetInfraUp("redis", false)

	return errors.Join(errs...)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

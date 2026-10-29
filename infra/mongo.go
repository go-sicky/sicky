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
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// MongoConfig is a infra component.
type MongoConfig struct {
	URI string `json:"uri" mapstructure:"uri" yaml:"uri"`
	DB  string `json:"db"  mapstructure:"db"  yaml:"db"`
	// MaxPoolSize caps driver connections. 0 keeps the driver default
	// (100).
	MaxPoolSize uint64 `json:"max_pool_size" mapstructure:"max_pool_size" yaml:"max_pool_size"`
	// ConnectTimeoutSec bounds initial dial/server selection. 0 keeps the
	// driver default (30s); negative aborts startup.
	ConnectTimeoutSec int `json:"connect_timeout_sec" mapstructure:"connect_timeout_sec" yaml:"connect_timeout_sec"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete MongoDB configurations. They do not inherit
	// fields from the block above.
	Instances map[string]*MongoConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// ErrMongoURIEmpty aborts startup: a non-nil MongoConfig means "enable
// mongo", and an empty URI would otherwise fail late after a 5s ping
// timeout.
var (
	ErrMongoURIEmpty      = errors.New("infra: mongo uri is empty")
	ErrMongoOptionInvalid = errors.New("infra: mongo pool/timeout option is negative")
)

// Mongo is a shared infra value.
var Mongo *mongo.Client

// InitMongo is part of the public API.
func InitMongo(cfg *MongoConfig) (*mongo.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	client, err := initMongo(cfg)
	metrics.CountInfraInit("mongo", err)
	if err == nil && client == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("mongo", false)
	}

	return client, err
}

func initMongo(cfg *MongoConfig) (*mongo.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"mongo config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init mongo instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init mongo default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"mongo default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("mongo no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"mongo using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"mongo no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Mongo
	mu.RUnlock()

	var (
		topLevel   *mongo.Client
		topLevelDB string
	)

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newMongoClient(cfg, "mongo")
		if err != nil {
			logger.Logger.Error(
				"init mongo failed",
				"uri", redactDSN(cfg.URI),
				"error", err.Error(),
			)

			return nil, err
		}

		topLevelDB = effectiveMongoDB(cfg)
	}

	built := make(map[string]mongoInstance, len(names))
	for _, name := range names {
		if _, ok := mongoInstances.get(name); ok {
			logger.Logger.Warn("mongo instance already initialized, keeping existing client", "instance", name)

			continue
		}

		client, berr := newMongoClient(instances[name], instanceLabel("mongo", name, selection))
		if berr != nil {
			metrics.CountInfraInit("mongo:"+name, berr)
			logger.Logger.Error(
				"init mongo instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackMongoClients(topLevel, built)

			return nil, fmt.Errorf("mongo instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("mongo:"+name, nil)
		built[name] = mongoInstance{
			client: client,
			db:     effectiveMongoDB(instances[name]),
		}
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Mongo == nil {
			Mongo = topLevel
			mongoDBName = topLevelDB
			mongoDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("mongo already initialized, closing duplicate connection")
			closeMongoClient(topLevel, "mongo")
		}
	}

	for _, name := range names {
		instance, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := mongoInstances.putFirst(name, instance); !stored {
			logger.Logger.Warn("mongo instance already initialized, closing duplicate connection", "instance", name)
			closeMongoClient(instance.client, "mongo:"+name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"mongo instance initialized",
			"instance", name,
			"uri", redactDSN(instances[name].URI),
			"db", instance.db,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if instance, ok := mongoInstances.get(selection.Instance); ok && instance.client != nil {
			mu.Lock()
			if Mongo == nil {
				Mongo = instance.client
				mongoDBName = instance.db
				mongoDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	client := Mongo
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"mongo initialized",
			"uri", redactDSN(cfg.URI),
			"db", topLevelDB,
		)
	}

	return client, nil
}

// newMongoClient builds, monitors and pings one client. It touches no
// package-level state, so a build that fails leaves nothing registered. The
// label names the metric series: "mongo" for the shared singleton,
// "mongo:<name>" for a named instance.
func newMongoClient(cfg *MongoConfig, label string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(cfg.URI)
	if cfg.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	}

	if cfg.ConnectTimeoutSec > 0 {
		clientOpts.SetConnectTimeout(time.Duration(cfg.ConnectTimeoutSec) * time.Second)
	}

	// Command monitor: every operation lands in sicky_infra_ops with
	// server-reported latency, no call-site changes needed.
	clientOpts.SetMonitor(&event.CommandMonitor{
		Succeeded: func(_ context.Context, e *event.CommandSucceededEvent) {
			metrics.InfraOpsTotal.WithLabelValues(label, e.CommandName, "ok").Inc()
			metrics.InfraOpDuration.WithLabelValues(label, e.CommandName).Observe(e.Duration.Seconds())
		},
		Failed: func(_ context.Context, e *event.CommandFailedEvent) {
			metrics.InfraOpsTotal.WithLabelValues(label, e.CommandName, "error").Inc()
			metrics.InfraOpDuration.WithLabelValues(label, e.CommandName).Observe(e.Duration.Seconds())
		},
	})

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		logger.Logger.Error(
			"mongo connect failed",
			"instance", label,
			"uri", redactDSN(cfg.URI),
			"error", err.Error(),
		)

		return nil, err
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Logger.Error(
			"mongo ping failed",
			"instance", label,
			"uri", redactDSN(cfg.URI),
			"error", err.Error(),
		)

		// Disconnect the handle opened above instead of leaking it.
		closeMongoClient(client, label)

		return nil, err
	}

	return client, nil
}

// closeMongoClient disconnects one client and logs a failure instead of
// returning it: the caller is already on an error or duplicate path.
func closeMongoClient(client *mongo.Client, label string) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	if derr := client.Disconnect(ctx); derr != nil {
		logger.Logger.Error(
			"mongo disconnect failed",
			"instance", label,
			"error", derr.Error(),
		)
	}
}

// rollbackMongoClients disconnects everything one init call built but will not
// register, so a failed build leaves no sockets or "up" series behind.
func rollbackMongoClients(defaultClient *mongo.Client, built map[string]mongoInstance) {
	closeMongoClient(defaultClient, "mongo")

	for name, instance := range built {
		closeMongoClient(instance.client, "mongo:"+name)
		metrics.SetInfraUp("mongo:"+name, false)
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *MongoConfig) Ensure() *MongoConfig {
	if c == nil {
		c = new(MongoConfig)
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *MongoConfig) Validate() error {
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
func (c *MongoConfig) validateShape() error {
	if c == nil {
		return nil
	}

	if c.ConnectTimeoutSec < 0 {
		return ErrMongoOptionInvalid
	}

	return nil
}

// validateIdentity requires the field that addresses the server.
func (c *MongoConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.URI) == "" {
		return ErrMongoURIEmpty
	}

	return nil
}

func (c *MongoConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *MongoConfig) resolveInstances() (resolved map[string]*MongoConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*MongoConfig, len(names))
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

// GetMongoDBOf returns a database handle for a named instance. An empty
// instance name selects the shared singleton, so it behaves exactly like
// GetMongoDB. An explicit name wins, then the instance's effective configured
// database; an unknown instance or an unresolvable name returns nil.
func GetMongoDBOf(instance string, name ...string) *mongo.Database {
	id := normalizeInstanceName(instance)
	if id == "" {
		return GetMongoDB(name...)
	}

	inst, ok := mongoInstances.get(id)
	if !ok || inst.client == nil {
		return nil
	}

	if len(name) > 0 && strings.TrimSpace(name[0]) != "" {
		return inst.client.Database(strings.TrimSpace(name[0]))
	}

	if inst.db != "" {
		return inst.client.Database(inst.db)
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

// ShutdownMongo disconnects the shared mongo client and every named instance,
// then clears their references. Named instances are detached before closing so
// a reader cannot observe a closed handle through the registry; a default
// alias is disconnected exactly once. One 5s deadline bounds the component.
func ShutdownMongo() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := mongoInstances.clear()

	mu.Lock()
	alias := mongoDefaultInstance
	defaultClient := Mongo
	Mongo = nil
	mongoDBName = ""
	mongoDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error

	for name, instance := range instances {
		if instance.client == nil {
			continue
		}

		if cerr := instance.client.Disconnect(ctx); cerr != nil {
			errs = append(errs, fmt.Errorf("mongo instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("mongo:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultClient != nil {
		if cerr := defaultClient.Disconnect(ctx); cerr != nil {
			errs = append(errs, fmt.Errorf("mongo: %w", cerr))
		}
	}

	metrics.SetInfraUp("mongo", false)

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

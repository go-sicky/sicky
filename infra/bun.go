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
 * @file bun.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/godoes/gorm-dameng/dm8"
	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/oracledialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// BunConfig is a infra component.
type BunConfig struct {
	Driver string `json:"driver" mapstructure:"driver" yaml:"driver"`
	DSN    string `json:"dsn"    mapstructure:"dsn"    yaml:"dsn"`
	Debug  bool   `json:"debug"  mapstructure:"debug"  yaml:"debug"`
	// Verbose enables full query logging including bound arguments.
	// Arguments may contain secrets: keep false in production.
	Verbose bool `json:"verbose" mapstructure:"verbose" yaml:"verbose"`
	// SlowDuration is a threshold placeholder, not a duration in
	// milliseconds: it only gates the query hook (>0 enables it, <=0
	// leaves it off) because bundebug v1.2.x exposes no threshold option.
	// The number itself is never compared against a query's latency, so
	// setting it does not make slow-query logging selective.
	//
	// Negative aborts startup, matching every other int here — a negative
	// value used to mean "hook off" silently, which is indistinguishable
	// from omitting the key.
	SlowDuration int `json:"slow_duration" mapstructure:"slow_duration" yaml:"slow_duration"`
	// Connection pool. Zero means driver default (unlimited open
	// connections); negative aborts startup.
	MaxOpenConns       int `json:"max_open_conns"         mapstructure:"max_open_conns"         yaml:"max_open_conns"`
	MaxIdleConns       int `json:"max_idle_conns"         mapstructure:"max_idle_conns"         yaml:"max_idle_conns"`
	ConnMaxLifetimeSec int `json:"conn_max_lifetime_sec"  mapstructure:"conn_max_lifetime_sec"  yaml:"conn_max_lifetime_sec"`
	ConnMaxIdleTimeSec int `json:"conn_max_idle_time_sec" mapstructure:"conn_max_idle_time_sec" yaml:"conn_max_idle_time_sec"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete SQL configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*BunConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// Bun is a shared infra value.
var Bun *bun.DB

// ErrBunDSNEmpty aborts startup: a non-nil BunConfig means "enable SQL"
// and an empty DSN would otherwise dial nowhere and fail late.
// ErrBunUnsupportedDriver aborts startup instead of silently falling back
// to PostgreSQL on a typo'd driver name.
// ErrBunDriverMissing aborts startup when driver was left empty and the DSN
// carries no unambiguous signal to derive it from — a distinct failure from
// a misspelled driver, so the operator knows to write driver explicitly.
// ErrBunPoolInvalid aborts startup on negative pool/lifetime settings.
var (
	ErrBunDSNEmpty          = errors.New("infra: bun dsn is empty")
	ErrBunUnsupportedDriver = errors.New("infra: bun unsupported driver")
	ErrBunDriverMissing     = errors.New("infra: bun driver missing")
	ErrBunPoolInvalid       = errors.New("infra: bun pool setting is negative")
	ErrBunSlowInvalid       = errors.New("infra: bun slow_duration is negative")
	// ErrBunUnavailable marks a transaction request before Bun initialized.
	ErrBunUnavailable = errors.New("infra: bun is not initialized")
)

// Driver names as written in config, matched case-insensitively. "postgresql"
// and "pg" are accepted aliases of "postgres"; detectBunDriver always derives
// the canonical name.
const (
	driverPostgres   = "postgres"
	driverPostgresql = "postgresql"
	driverMySQL      = "mysql"
	driverMSSQL      = "mssql"
	driverSQLite     = "sqlite"
	driverDM         = "dm"
)

// detectBunDriver derives the driver name from an unambiguous DSN form and
// returns "" when the DSN carries no signal. It only recognizes shapes that
// cannot mean two databases at once — a URL scheme, or mysql's scheme-less
// user:pass@tcp(...) form — and deliberately refuses bare paths and key/value
// strings, which is why an operator may still have to set driver explicitly.
//
//	postgres:// postgresql:// unix://  -> postgres  (pgdriver accepts no other scheme)
//	dm://                            -> dm        (the driver rejects anything else)
//	sqlserver://                     -> mssql
//	user:pass@tcp(...) user:pass@unix(...) -> mysql (go-sql-driver/mysql has no scheme)
//	:memory: file:                   -> sqlite
func detectBunDriver(dsn string) string {
	s := strings.TrimSpace(dsn)
	if s == "" {
		return ""
	}

	if s == ":memory:" || strings.HasPrefix(strings.ToLower(s), "file:") {
		return driverSQLite
	}

	if scheme, _, ok := strings.Cut(s, "://"); ok {
		switch strings.ToLower(scheme) {
		case driverPostgres, driverPostgresql, "unix":
			return driverPostgres
		case driverDM:
			return driverDM
		case "sqlserver":
			return driverMSSQL
		}
	}

	// go-sql-driver/mysql DSNs carry no scheme: user:pass@tcp(host:3306)/db.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "@tcp(") || strings.Contains(lower, "@unix(") {
		return driverMySQL
	}

	return ""
}

// bunMetricsHook records every query into sicky_infra_ops. It is always
// attached (unlike the debug-only bundebug hook) so SQL latency/error is
// observed in production without verbose logging. label names the component
// series: "bun" for the shared singleton, "bun:<name>" for a named instance.
type bunMetricsHook struct {
	label string
}

// component returns the metric label, defaulting to the shared series when the
// hook was built without one.
func (h bunMetricsHook) component() string {
	if h.label == "" {
		return "bun"
	}

	return h.label
}

func (h bunMetricsHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (h bunMetricsHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	op := event.Operation()
	if op == "" {
		op = "query"
	}

	metrics.ObserveInfraOp(h.component(), op, event.StartTime, event.Err)
}

// InitBun is part of the public API.
func InitBun(cfg *BunConfig) (*bun.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	db, err := initBun(cfg)
	metrics.CountInfraInit("bun", err)
	if err == nil && db == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("bun", false)
	}

	return db, err
}

func initBun(cfg *BunConfig) (*bun.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"database config invalid",
			"driver", cfg.Driver,
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init bun instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init bun default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"database default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("database no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"database using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"database no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Bun
	mu.RUnlock()

	var topLevel *bun.DB

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newBunClient(cfg, "bun")
		if err != nil {
			logger.Logger.Error(
				"init bun failed",
				"driver", cfg.Driver,
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]*bun.DB, len(names))
	for _, name := range names {
		if _, ok := bunInstances.get(name); ok {
			logger.Logger.Warn("database instance already initialized, keeping existing client", "instance", name)

			continue
		}

		db, berr := newBunClient(instances[name], instanceLabel("bun", name, selection))
		if berr != nil {
			metrics.CountInfraInit("bun:"+name, berr)
			logger.Logger.Error(
				"init bun instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackBunClients(topLevel, built)

			return nil, fmt.Errorf("bun instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("bun:"+name, nil)
		built[name] = db
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Bun == nil {
			Bun = topLevel
			bunDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("database already initialized, closing duplicate connection")
			closeBunClient(topLevel, "bun")
		}
	}

	for _, name := range names {
		db, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := bunInstances.putFirst(name, db); !stored {
			logger.Logger.Warn("database instance already initialized, closing duplicate connection", "instance", name)
			closeBunClient(db, "bun:"+name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"database instance initialized",
			"instance", name,
			"driver", instances[name].Driver,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if db, ok := bunInstances.get(selection.Instance); ok && db != nil {
			mu.Lock()
			if Bun == nil {
				Bun = db
				bunDefaultInstance = selection.Instance
			}

			mu.Unlock()
		}
	}

	mu.RLock()
	db := Bun
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"database initialized",
			"driver", cfg.Driver,
			"debug", cfg.Debug,
			"max_open_conns", cfg.MaxOpenConns,
		)
	}

	return db, nil
}

// newBunClient opens, pings and configures one Bun handle. It touches no
// package-level state, so a build that fails leaves nothing registered.
func newBunClient(cfg *BunConfig, label string) (*bun.DB, error) {
	var (
		sqldb *sql.DB
		err   error
		db    *bun.DB
	)

	switch strings.ToLower(cfg.Driver) {
	case driverMySQL:
		// MySQL
		sqldb, err = sql.Open("mysql", cfg.DSN)
		if err != nil {
			logger.Logger.Error(
				"database open failed",
				"instance", label,
				"driver", cfg.Driver,
				"error", err.Error(),
			)

			return nil, err
		}

		db = bun.NewDB(sqldb, mysqldialect.New())
	case driverMSSQL:
		// MS-SQLServer
		sqldb, err = sql.Open("sqlserver", cfg.DSN)
		if err != nil {
			logger.Logger.Error(
				"database open failed",
				"instance", label,
				"driver", cfg.Driver,
				"error", err.Error(),
			)

			return nil, err
		}

		db = bun.NewDB(sqldb, mssqldialect.New())
	case driverSQLite:
		// SQLite
		sqldb, err = sql.Open("sqlite3", cfg.DSN)
		if err != nil {
			logger.Logger.Error(
				"database open failed",
				"instance", label,
				"driver", cfg.Driver,
				"error", err.Error(),
			)

			return nil, err
		}

		db = bun.NewDB(sqldb, sqlitedialect.New())
	case driverDM:
		// DaMeng (uses Oracle dialect as fallback)
		sqldb, err = sql.Open("dm", cfg.DSN)
		if err != nil {
			logger.Logger.Error(
				"database open failed",
				"instance", label,
				"driver", cfg.Driver,
				"error", err.Error(),
			)

			return nil, err
		}

		db = bun.NewDB(sqldb, oracledialect.New())
	case driverPostgres, driverPostgresql, "pg":
		// PostgreSQL
		sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DSN)))
		db = bun.NewDB(sqldb, pgdialect.New())
	default:
		// Unreachable after Validate, kept as defense in depth.
		return nil, ErrBunUnsupportedDriver
	}

	// Ping with a timeout: database/sql dials lazily and Ping without a
	// deadline could hang startup indefinitely.
	pctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	err = db.PingContext(pctx)
	if err != nil {
		logger.Logger.Error(
			"database initialize failed",
			"instance", label,
			"driver", cfg.Driver,
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking it.
		closeBunClient(db, label)

		return nil, err
	}

	// Connection pool: database/sql defaults to unlimited open
	// connections, which can overwhelm the DB under load. Zero keeps the
	// driver default (unbounded) for backward compatibility — set
	// max_open_conns explicitly in production.
	if cfg.MaxOpenConns <= 0 {
		logger.Logger.Warn(
			"database connection pool unbounded; set max_open_conns to cap it",
			"instance", label,
			"driver", cfg.Driver,
		)
	} else {
		sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetimeSec > 0 {
		sqldb.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeSec) * time.Second)
	}

	if cfg.ConnMaxIdleTimeSec > 0 {
		sqldb.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeSec) * time.Second)
	}

	// Debug logger. Verbose logs bound query arguments, which may contain
	// secrets — keep Verbose false in production.
	if cfg.Verbose {
		logger.Logger.Warn(
			"database verbose query logging enabled; bound arguments may contain secrets",
			"instance", label,
			"driver", cfg.Driver,
		)
	}

	if cfg.Debug {
		db.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithEnabled(true),
			bundebug.WithVerbose(cfg.Verbose),
		))
	} else if cfg.SlowDuration > 0 {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithEnabled(true)))
	}

	// Metrics hook is unconditional: production needs query RED too.
	db.AddQueryHook(bunMetricsHook{label: label})

	return db, nil
}

// closeBunClient closes one client and logs a failure instead of returning it:
// the caller is already on an error or duplicate path.
func closeBunClient(db *bun.DB, label string) {
	if db == nil {
		return
	}

	if cerr := db.Close(); cerr != nil {
		logger.Logger.Error(
			"database close failed",
			"instance", label,
			"error", cerr.Error(),
		)
	}
}

// rollbackBunClients closes everything one init call built but will not
// register, so a failed build leaves no pools, sockets or "up" series behind.
func rollbackBunClients(defaultDB *bun.DB, built map[string]*bun.DB) {
	closeBunClient(defaultDB, "bun")

	for name, db := range built {
		closeBunClient(db, "bun:"+name)
		metrics.SetInfraUp("bun:"+name, false)
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
// A blank Driver is derived from the DSN when the DSN carries an unambiguous
// signal — the operator may then configure only dsn. The derived name is written
// back so logs, /config and initBun all see the same driver; an underivable one
// stays empty and Validate rejects it.
func (c *BunConfig) Ensure() *BunConfig {
	if c == nil {
		c = new(BunConfig)
	}

	if strings.TrimSpace(c.Driver) == "" {
		c.Driver = detectBunDriver(c.DSN)
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *BunConfig) Validate() error {
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
// identity, so an instance can be rejected before anything dials. An explicit
// driver outside the allow-list is a typo, not an absent driver.
func (c *BunConfig) validateShape() error {
	if c == nil {
		return nil
	}

	if c.SlowDuration < 0 {
		return ErrBunSlowInvalid
	}

	// Pool/lifetime settings: negative aborts, zero keeps driver default.
	if c.MaxOpenConns < 0 || c.MaxIdleConns < 0 ||
		c.ConnMaxLifetimeSec < 0 || c.ConnMaxIdleTimeSec < 0 {
		return ErrBunPoolInvalid
	}

	// Resolve the same way Ensure does, without mutating: Validate must
	// agree with initBun even when a caller validates a raw config.
	driver := strings.TrimSpace(c.Driver)
	if driver == "" {
		driver = detectBunDriver(c.DSN)
	}

	if driver == "" {
		// An absent driver is an identity problem, reported by
		// validateIdentity once the DSN is known to be present.
		return nil
	}

	switch strings.ToLower(driver) {
	case driverPostgres, driverPostgresql, "pg", driverMySQL, driverMSSQL, driverSQLite, driverDM:
		return nil
	default:
		return ErrBunUnsupportedDriver
	}
}

// validateIdentity requires the fields that address the database.
func (c *BunConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.DSN) == "" {
		return ErrBunDSNEmpty
	}

	driver := strings.TrimSpace(c.Driver)
	if driver == "" {
		driver = detectBunDriver(c.DSN)
	}

	if driver == "" {
		return ErrBunDriverMissing
	}

	return nil
}

func (c *BunConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *BunConfig) resolveInstances() (resolved map[string]*BunConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*BunConfig, len(names))
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

type bunTxContextKey struct{}

// WithBunTx returns a context carrying db as the current Bun executor. A nil
// executor leaves the context unchanged.
func WithBunTx(ctx context.Context, db bun.IDB) context.Context {
	if isNilBunIDB(db) {
		return ctx
	}

	return context.WithValue(ctx, bunTxContextKey{}, db)
}

// isNilBunIDB reports whether db is nil even when it is a typed nil stored in
// the bun.IDB interface. A plain db == nil check misses that case and stores a
// concrete nil in the context, which BunDB would then hand to a caller.
func isNilBunIDB(db bun.IDB) bool {
	if db == nil {
		return true
	}

	value := reflect.ValueOf(db)
	kind := value.Kind()
	nilable := kind == reflect.Chan ||
		kind == reflect.Func ||
		kind == reflect.Interface ||
		kind == reflect.Map ||
		kind == reflect.Pointer ||
		kind == reflect.Slice

	return nilable && value.IsNil()
}

// BunDB returns the Bun executor carried by ctx, or the package-level Bun
// handle when no transaction is active. It returns a nil interface when Bun
// is uninitialized, so callers can compare the result with nil.
func BunDB(ctx context.Context) bun.IDB {
	if ctx != nil {
		if db, ok := ctx.Value(bunTxContextKey{}).(bun.IDB); ok && !isNilBunIDB(db) {
			return db
		}
	}

	db := GetBun()
	if db == nil {
		return nil
	}

	return db
}

// RunInBunTx runs fn inside a Bun transaction and injects the transaction into
// the context passed to fn. The transaction is committed when fn returns nil
// and rolled back otherwise.
func RunInBunTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context) error) error {
	db := GetBun()
	if db == nil {
		return ErrBunUnavailable
	}

	return db.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		return fn(WithBunTx(ctx, tx))
	})
}

// RunInBunTxIns runs fn inside a transaction on the named Bun instance. An
// empty instance name selects the shared singleton, so it then behaves exactly
// like RunInBunTx.
func RunInBunTxIns(ctx context.Context, instance string, opts *sql.TxOptions, fn func(context.Context) error) error {
	db := GetBun(instance)
	if db == nil {
		return ErrBunUnavailable
	}

	return db.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		return fn(WithBunTx(ctx, tx))
	})
}

// ShutdownBun closes the shared bun and every named instance, then clears
// their references. Named instances are detached before closing so a reader
// cannot observe a closed handle through the registry; a default alias is
// closed exactly once.
//
//nolint:dupl // each component owns its globals; the detach/close/metrics shape is deliberately uniform
func ShutdownBun() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := bunInstances.clear()

	mu.Lock()
	alias := bunDefaultInstance
	defaultDB := Bun
	Bun = nil
	bunDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultDB == nil {
		return nil
	}

	var errs []error

	for name, db := range instances {
		if db == nil {
			continue
		}

		if cerr := db.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("bun instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("bun:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultDB != nil {
		if cerr := defaultDB.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("bun: %w", cerr))
		}
	}

	metrics.SetInfraUp("bun", false)

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

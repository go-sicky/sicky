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
// observed in production without verbose logging.
type bunMetricsHook struct{}

func (bunMetricsHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (bunMetricsHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	op := event.Operation()
	if op == "" {
		op = "query"
	}

	metrics.ObserveInfraOp("bun", op, event.StartTime, event.Err)
}

// InitBun is part of the public API.
func InitBun(cfg *BunConfig) (*bun.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	db, err := initBun(cfg)
	metrics.CountInfraInit("bun", err)

	return db, err
}

func initBun(cfg *BunConfig) (*bun.DB, error) {
	var (
		sqldb *sql.DB
		err   error
		db    *bun.DB
	)

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

	switch strings.ToLower(cfg.Driver) {
	case driverMySQL:
		// MySQL
		sqldb, err = sql.Open("mysql", cfg.DSN)
		if err != nil {
			logger.Logger.Error(
				"database open failed",
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
			"driver", cfg.Driver,
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking it.
		if cerr := db.Close(); cerr != nil {
			logger.Logger.Error(
				"database close after failed ping failed",
				"driver", cfg.Driver,
				"error", cerr.Error(),
			)
		}

		return nil, err
	}

	// Connection pool: database/sql defaults to unlimited open
	// connections, which can overwhelm the DB under load. Zero keeps the
	// driver default (unbounded) for backward compatibility — set
	// max_open_conns explicitly in production.
	if cfg.MaxOpenConns <= 0 {
		logger.Logger.Warn(
			"database connection pool unbounded; set max_open_conns to cap it",
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
	db.AddQueryHook(bunMetricsHook{})

	logger.Logger.Info(
		"database initialized",
		"driver", cfg.Driver,
		"debug", cfg.Debug,
		"max_open_conns", cfg.MaxOpenConns,
	)

	// Swap under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	// The success log moved out too: it is a synchronous write to the
	// handler and must not run under the global lock either.
	mu.Lock()

	existing := Bun
	if existing == nil {
		Bun = db
	}

	mu.Unlock()

	if existing != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("database already initialized, closing duplicate connection")
		if cerr := db.Close(); cerr != nil {
			logger.Logger.Error(
				"database duplicate close failed",
				"driver", cfg.Driver,
				"error", cerr.Error(),
			)
		}

		return existing, nil
	}

	return db, nil
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

// Validate rejects half-configured or illegal values.
func (c *BunConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.DSN) == "" {
		return ErrBunDSNEmpty
	}

	if c.SlowDuration < 0 {
		return ErrBunSlowInvalid
	}

	// Resolve the same way Ensure does, without mutating: Validate must
	// agree with initBun even when a caller validates a raw config.
	driver := strings.TrimSpace(c.Driver)
	if driver == "" {
		driver = detectBunDriver(c.DSN)
	}

	if driver == "" {
		return ErrBunDriverMissing
	}

	switch strings.ToLower(driver) {
	case driverPostgres, driverPostgresql, "pg", driverMySQL, driverMSSQL, driverSQLite, driverDM:
		// Pool/lifetime settings: negative aborts, zero keeps driver default.
		if c.MaxOpenConns < 0 || c.MaxIdleConns < 0 ||
			c.ConnMaxLifetimeSec < 0 || c.ConnMaxIdleTimeSec < 0 {
			return ErrBunPoolInvalid
		}

		return nil
	default:
		return ErrBunUnsupportedDriver
	}
}

type bunTxContextKey struct{}

// WithBunTx returns a context carrying db as the current Bun executor. A nil
// executor leaves the context unchanged.
func WithBunTx(ctx context.Context, db bun.IDB) context.Context {
	if db == nil {
		return ctx
	}

	return context.WithValue(ctx, bunTxContextKey{}, db)
}

// BunDB returns the Bun executor carried by ctx, or the package-level Bun
// handle when no transaction is active. It returns a nil interface when Bun
// is uninitialized, so callers can compare the result with nil.
func BunDB(ctx context.Context) bun.IDB {
	if ctx != nil {
		if db, ok := ctx.Value(bunTxContextKey{}).(bun.IDB); ok && db != nil {
			return db
		}
	}

	if Bun == nil {
		return nil
	}

	return Bun
}

// RunInBunTx runs fn inside a Bun transaction and injects the transaction into
// the context passed to fn. The transaction is committed when fn returns nil
// and rolled back otherwise.
func RunInBunTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context) error) error {
	if Bun == nil {
		return ErrBunUnavailable
	}

	return Bun.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		return fn(WithBunTx(ctx, tx))
	})
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

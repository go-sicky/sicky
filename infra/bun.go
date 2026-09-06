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
	// SlowDuration enables the query hook when >0, in milliseconds.
	// Reserved for threshold filtering; bundebug v1.2.x exposes no
	// threshold option, so the value itself only gates the hook.
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
// ErrBunPoolInvalid aborts startup on negative pool/lifetime settings.
var (
	ErrBunDSNEmpty          = errors.New("infra: bun dsn is empty")
	ErrBunUnsupportedDriver = errors.New("infra: bun unsupported driver")
	ErrBunPoolInvalid       = errors.New("infra: bun pool setting is negative")
)

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
	case "mysql":
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
	case "mssql":
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
	case "sqlite":
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
	case "dm":
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
	case "postgres", "postgresql", "pg":
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

	mu.Lock()
	defer mu.Unlock()
	if Bun != nil {
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

		return Bun, nil
	}

	Bun = db

	return db, nil
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *BunConfig) Ensure() *BunConfig {
	if c == nil {
		c = new(BunConfig)
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

	switch strings.ToLower(strings.TrimSpace(c.Driver)) {
	case "postgres", "postgresql", "pg", "mysql", "mssql", "sqlite", "dm":
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

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

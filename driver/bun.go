/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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
 * @package driver
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package driver

import (
	"database/sql"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type BunConfig struct {
	Driver string `json:"driver" yaml:"driver" mapstructure:"driver"`
	DSN    string `json:"dsn" yaml:"dsn" mapstructure:"dsn"`
}

var DB *bun.DB

func InitBun(cfg *BunConfig) (*bun.DB, error) {
	var (
		sqldb *sql.DB
		err   error
		db    *bun.DB
	)
	switch strings.ToLower(cfg.Driver) {
	case "mysql":
		sqldb, err = sql.Open("mysql", cfg.DSN)
		if err != nil {
			return nil, err
		}

		db = bun.NewDB(sqldb, mysqldialect.New())
	case "mssql":
		sqldb, err = sql.Open("sqlserver", cfg.DSN)
		if err != nil {
			return nil, err
		}

		db = bun.NewDB(sqldb, mssqldialect.New())
	case "sqlite":
		sqldb, err = sql.Open("sqlite3", cfg.DSN)
		if err != nil {
			return nil, err
		}

		db = bun.NewDB(sqldb, sqlitedialect.New())
	default:
		// PostgreSQL
		sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DSN)))
		db = bun.NewDB(sqldb, pgdialect.New())
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	DB = db

	return db, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

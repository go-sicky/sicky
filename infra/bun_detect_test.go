package infra

import (
	"errors"
	"testing"
)

// Only unambiguous DSN shapes may be derived: a URL scheme that exactly one of
// the five supported drivers accepts, or mysql's scheme-less @tcp(...) form.
// Everything else — bare paths, key/value strings, unknown schemes — must come
// back empty so the operator is told to set driver instead of being handed a
// guess that might point at the wrong database.
func TestDetectBunDriver(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"postgres url", "postgres://user:pass@localhost:5432/db?sslmode=disable", "postgres"},
		{"postgresql url", "postgresql://localhost/db", "postgres"},
		{"pgdriver unix socket", "unix://user:pass@db/var/run/postgresql/.s.PGSQL.5432", "postgres"},
		{"dameng", "dm://SYSDBA:SYSDBA@localhost:5236", "dm"},
		{"sqlserver url", "sqlserver://sa:pw@localhost:1433?database=master", "mssql"},
		{"mysql tcp", "user:pass@tcp(127.0.0.1:3306)/app?parseTime=true", "mysql"},
		{"mysql unix socket", "user:pass@unix(/var/run/mysqld/mysqld.sock)/app", "mysql"},
		{"sqlite memory", ":memory:", "sqlite"},
		{"sqlite file uri", "file:app.db?cache=shared", "sqlite"},
		{"blank", "   ", ""},
		{"bare path", "/var/lib/mysql/app", ""},
		{"path without scheme", "app.db", ""},
		{"ado mssql key values", "server=localhost;user id=sa;password=x;database=app", ""},
		{"unknown scheme", "oracle://host:1521/db", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectBunDriver(tc.dsn); got != tc.want {
				t.Errorf("detectBunDriver(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}

// Ensure is where the derivation lands, so that config.go, validateConfig and
// initBun — three callers of Ensure — see one driver value. An explicit driver
// must never be touched, whatever the DSN says.
func TestBunEnsureDerivesDriverFromDSN(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *BunConfig
		want string
	}{
		{"postgres url", &BunConfig{DSN: "postgres://localhost/db"}, "postgres"},
		{"mysql tcp", &BunConfig{DSN: "user:pass@tcp(localhost:3306)/app"}, "mysql"},
		{"sqlite memory", &BunConfig{DSN: ":memory:"}, "sqlite"},
		{"underivable stays empty", &BunConfig{DSN: "/var/lib/mysql/app"}, ""},
		{"explicit wins over dsn", &BunConfig{Driver: "pg", DSN: "dm://SYSDBA:SYSDBA@localhost:5236"}, "pg"},
		{"explicit kept as written", &BunConfig{Driver: "postgres", DSN: "postgres://localhost/db"}, "postgres"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Ensure().Driver; got != tc.want {
				t.Errorf("Ensure().Driver = %q, want %q", got, tc.want)
			}
		})
	}
}

// The whole point of the change: a dsn-only config must now validate. Both
// Validate call sites (validateConfig in sicky.go and initBun) chain Ensure
// first, so this is the same path production takes.
func TestBunValidateAcceptsDriverlessDSN(t *testing.T) {
	cfg := &BunConfig{DSN: "postgres://localhost/db"}
	if err := cfg.Ensure().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for a derivable driver", err)
	}

	if cfg.Driver != "postgres" {
		t.Errorf("Ensure did not write back the derived driver, got %q", cfg.Driver)
	}
}

// ...and an underivable one must still abort, with the sentinel that says
// "write driver yourself" rather than the misspelled-driver sentinel.
func TestBunValidateRejectsUnderivableDriver(t *testing.T) {
	cfg := &BunConfig{DSN: "/var/lib/mysql/app"}
	if err := cfg.Ensure().Validate(); !errors.Is(err, ErrBunDriverMissing) {
		t.Errorf("Validate() = %v, want %v", err, ErrBunDriverMissing)
	}
}

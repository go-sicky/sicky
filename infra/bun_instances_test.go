/**
 * @file bun_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/uptrace/bun"
)

func TestBunConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {MaxOpenConns: 1},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrBunDSNEmpty) {
		t.Fatalf("Validate() = %v, want ErrBunDSNEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.reporting") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestBunConfigRejectsNestedInstances(t *testing.T) {
	cfg := &BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {
				DSN:       ":memory:",
				Instances: map[string]*BunConfig{"nested": {DSN: ":memory:"}},
			},
		},
	}

	if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
		t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
	}
}

// Shape errors are caught per instance and name the instance, so an operator
// does not have to guess which block is wrong.
func TestBunConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:", MaxOpenConns: -1},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrBunPoolInvalid) {
		t.Fatalf("Validate() = %v, want ErrBunPoolInvalid", err)
	}

	if !strings.Contains(err.Error(), "instances.reporting") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// resolveInstances validates a copy: Ensure writes the derived driver back,
// and the caller's map is the caller's property.
func TestBunConfigInstancesDoNotMutateCaller(t *testing.T) {
	cfg := &BunConfig{
		Instances: map[string]*BunConfig{
			"reporting": {DSN: "postgres://localhost/db"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["reporting"].Driver; got != "" {
		t.Fatalf("Validate() wrote %q into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "reporting" {
		t.Fatalf("resolveInstances() names = %v, want [reporting]", names)
	}

	if got := instances["reporting"].Driver; got != driverPostgres {
		t.Fatalf("resolved instance driver = %q, want %q", got, driverPostgres)
	}
}

func TestBunValidateDefaultSelection(t *testing.T) {
	instances := map[string]*BunConfig{
		"reporting": {DSN: ":memory:"},
		"audit":     {DSN: ":memory:"},
	}

	if err := (&BunConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&BunConfig{DefaultInstance: "reporting", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&BunConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&BunConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&BunConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&BunConfig{NoDefault: true, DefaultInstance: "reporting", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

func TestInitBunWithNamedInstances(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	defaultDB, err := InitBun(&BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
			"audit":     {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	if defaultDB == nil || GetBun() != defaultDB {
		t.Fatal("GetBun() must return the complete top-level client")
	}

	for name, db := range map[string]*bun.DB{
		"default":   defaultDB,
		"reporting": GetBun("reporting"),
		"audit":     GetBun("audit"),
	} {
		if db == nil {
			t.Fatalf("GetBun(%s) = nil, want a live client", name)
		}

		if err := db.PingContext(context.Background()); err != nil {
			t.Errorf("%s client ping = %v, want nil", name, err)
		}
	}

	if GetBun("reporting") == defaultDB {
		t.Fatal("a named instance must be its own handle, not the shared singleton")
	}

	if GetBunInstances()["audit"] != GetBun("audit") {
		t.Fatal("the named snapshot must expose the registered client")
	}

	reportingDB := GetBun("reporting")
	ClearBun("reporting")
	if GetBun("reporting") != nil {
		t.Fatal("ClearBun(reporting) left the instance registered")
	}

	if GetBun() == nil {
		t.Fatal("ClearBun(reporting) must not clear the top-level default")
	}

	reportingDB.Close()

	auditDB := GetBun("audit")
	ClearBunAll()
	defaultDB.Close()
	auditDB.Close()
	if GetBun() != nil || GetBun("audit") != nil {
		t.Fatal("ClearBunAll must clear the default and every named instance")
	}
}

func TestInitBunUsesFirstSortedInstanceAsDefault(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	db, err := InitBun(&BunConfig{
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
			"audit":     {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	if db == nil || GetBun() != db {
		t.Fatal("InitBun must return the first sorted instance as the default")
	}

	// "audit" sorts before "reporting", so it becomes the shared singleton.
	if GetBun("audit") != db {
		t.Fatal("the sorted first instance (audit) must be the default")
	}

	if GetBun("reporting") == nil || GetBun("reporting") == db {
		t.Fatal("reporting must remain a distinct named client")
	}
}

func TestInitBunDefaultInstanceAlias(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	client, err := InitBun(&BunConfig{
		DefaultInstance: "reporting",
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
			"audit":     {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	if client == nil || GetBun() != client || GetBun() != GetBun("reporting") {
		t.Fatal("GetBun() must be the reporting alias")
	}

	if GetBun("audit") == client {
		t.Fatal("the alias must not shadow a different instance")
	}

	mu.RLock()
	alias := bunDefaultInstance
	mu.RUnlock()

	if alias != "reporting" {
		t.Fatalf("bunDefaultInstance = %q, want reporting", alias)
	}

	// The alias is also a named instance, so Shutdown closes it through the
	// named loop instead of closing the shared reference a second time.
	if defaultNeedsClose(alias, GetBunInstances()) {
		t.Fatal("the default alias must not be closed a second time")
	}

	reporting := GetBun("reporting")
	ClearBun("reporting")
	if GetBun() != nil {
		t.Fatal("clearing the default alias must clear the shared reference")
	}

	reporting.Close()

	if GetBun("audit") == nil {
		t.Fatal("clearing reporting must not clear audit")
	}
}

// InitBun must install an alias only into a free singleton slot: the shared
// reference is first-wins, exactly like the named registry.
func TestInitBunKeepsExistingClients(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	first, err := InitBun(&BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("first InitBun() = %v, want nil", err)
	}

	existing := GetBun("reporting")

	second, err := InitBun(&BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("second InitBun() = %v, want nil", err)
	}

	if second != first {
		t.Fatal("a duplicate init must return the existing singleton, not a rebuild")
	}

	if GetBun("reporting") != existing {
		t.Fatal("an already-registered named instance must not be rebuilt")
	}
}

func TestInitBunNoDefault(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	client, err := InitBun(&BunConfig{
		NoDefault: true,
		Instances: map[string]*BunConfig{
			"reporting": {DSN: ":memory:"},
		},
	})
	if err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	if client != nil {
		t.Fatalf("InitBun() = %v, want nil with no_default", client)
	}

	if GetBun() != nil {
		t.Fatal("GetBun() must be nil with no_default")
	}

	if GetBun("reporting") == nil {
		t.Fatal("GetBun(reporting) = nil, want the named instance")
	}
}

func TestInitBunNoDefaultWithoutInstances(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	if _, err := InitBun(&BunConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitBun() = %v, want ErrInstancesEmpty", err)
	}
}

// A named build that fails after the top-level client succeeded must leave
// nothing behind: the half-installed component would advertise a default that
// no later init could replace.
func TestInitBunRollsBackBuiltClientsOnInstanceFailure(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	_, err := InitBun(&BunConfig{
		Driver: driverSQLite,
		DSN:    ":memory:",
		Instances: map[string]*BunConfig{
			"broken": {DSN: "postgres://127.0.0.1:1/db?sslmode=disable"},
		},
	})
	if err == nil {
		t.Fatal("InitBun() = nil, want the instance build failure")
	}

	if !strings.Contains(err.Error(), "bun instance broken") {
		t.Fatalf("InitBun() = %v, want the failing instance named", err)
	}

	if GetBun() != nil {
		t.Fatal("the top-level client must be rolled back when a named build fails")
	}

	if len(GetBunInstances()) != 0 {
		t.Fatalf("GetBunInstances() = %v, want none after a rolled-back init", GetBunInstances())
	}
}

// Shutdown closes the alias through the named loop; a second close of the
// shared reference must be skipped (defaultNeedsClose), which is what keeps a
// database/sql handle from being closed twice.
func TestShutdownBunClosesDefaultAliasOnce(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	if _, err := InitBun(&BunConfig{
		DefaultInstance: "primary",
		Instances: map[string]*BunConfig{
			"primary": {DSN: ":memory:"},
		},
	}); err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	if err := ShutdownBun(); err != nil {
		t.Fatalf("ShutdownBun() = %v, want nil", err)
	}

	if GetBun() != nil || GetBun("primary") != nil {
		t.Fatal("ShutdownBun must clear the alias and the named instance")
	}
}

func TestRunInBunTxIns(t *testing.T) {
	ShutdownBun()
	t.Cleanup(func() { ShutdownBun() })

	if _, err := InitBun(&BunConfig{
		Driver:       driverSQLite,
		DSN:          ":memory:",
		MaxOpenConns: 1,
		Instances: map[string]*BunConfig{
			"reporting": {Driver: driverSQLite, DSN: ":memory:", MaxOpenConns: 1},
		},
	}); err != nil {
		t.Fatalf("InitBun() = %v, want nil", err)
	}

	err := RunInBunTxIns(context.Background(), "reporting", nil, func(ctx context.Context) error {
		db := BunDB(ctx)
		if db == nil {
			t.Fatal("BunDB(ctx) = nil inside the instance transaction")
		}

		_, err := db.ExecContext(ctx, "CREATE TABLE tx_probe (id INTEGER)")

		return err
	})
	if err != nil {
		t.Fatalf("RunInBunTxIns() = %v, want nil", err)
	}

	// The committed table must exist on the named instance, not on the
	// shared singleton.
	var count int
	if err := GetBun("reporting").QueryRowContext(context.Background(), "SELECT COUNT(*) FROM tx_probe").Scan(&count); err != nil {
		t.Fatalf("the committed table is missing from the named instance: %v", err)
	}

	called := false
	err = RunInBunTxIns(context.Background(), "missing", nil, func(context.Context) error {
		called = true

		return nil
	})
	if !errors.Is(err, ErrBunUnavailable) {
		t.Fatalf("RunInBunTxIns(missing) = %v, want ErrBunUnavailable", err)
	}

	if called {
		t.Fatal("RunInBunTxIns called the callback for an unknown instance")
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

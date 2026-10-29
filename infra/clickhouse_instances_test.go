/**
 * @file clickhouse_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"errors"
	"strings"
	"testing"
)

func TestClickHouseConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &ClickHouseConfig{
		DSN: "clickhouse://127.0.0.1:9000/default",
		Instances: map[string]*ClickHouseConfig{
			"analytics": {},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrClickHouseDSNEmpty) {
		t.Fatalf("Validate() = %v, want ErrClickHouseDSNEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.analytics") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestClickHouseConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *ClickHouseConfig
	}{
		{
			name: "instances",
			instance: &ClickHouseConfig{
				DSN:       "clickhouse://127.0.0.1:9000/analytics",
				Instances: map[string]*ClickHouseConfig{"nested": {DSN: "clickhouse://127.0.0.1:9000/nested"}},
			},
		},
		{
			name:     "default_instance",
			instance: &ClickHouseConfig{DSN: "clickhouse://127.0.0.1:9000/analytics", DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &ClickHouseConfig{DSN: "clickhouse://127.0.0.1:9000/analytics", NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ClickHouseConfig{
				DSN:       "clickhouse://127.0.0.1:9000/default",
				Instances: map[string]*ClickHouseConfig{"analytics": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// resolveInstances returns copies: the caller keeps ownership of its map.
func TestClickHouseConfigInstancesDoNotMutateCaller(t *testing.T) {
	cfg := &ClickHouseConfig{
		DSN: "clickhouse://127.0.0.1:9000/default",
		Instances: map[string]*ClickHouseConfig{
			"analytics": {DSN: "clickhouse://127.0.0.1:9000/analytics"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "analytics" {
		t.Fatalf("resolveInstances() names = %v, want [analytics]", names)
	}

	if instances["analytics"] == cfg.Instances["analytics"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if cfg.Instances["analytics"].Instances != nil {
		t.Fatal("Validate() mutated the caller's instance config")
	}
}

func TestClickHouseValidateDefaultSelection(t *testing.T) {
	instances := map[string]*ClickHouseConfig{
		"analytics": {DSN: "clickhouse://127.0.0.1:9000/analytics"},
		"audit":     {DSN: "clickhouse://127.0.0.1:9000/audit"},
	}

	if err := (&ClickHouseConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&ClickHouseConfig{DefaultInstance: "analytics", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&ClickHouseConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&ClickHouseConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&ClickHouseConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&ClickHouseConfig{NoDefault: true, DefaultInstance: "analytics", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

func TestInitClickHouseNoDefaultWithoutInstances(t *testing.T) {
	ShutdownClickHouse()
	t.Cleanup(func() { ShutdownClickHouse() })

	if _, err := InitClickHouse(&ClickHouseConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitClickHouse() = %v, want ErrInstancesEmpty", err)
	}
}

// InitClickHouse must fail fast and register nothing: an unreachable server is
// an ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitClickHouseFailFast(t *testing.T) {
	const dsn = "clickhouse://127.0.0.1:1/default"

	for _, tc := range []struct {
		name string
		cfg  *ClickHouseConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &ClickHouseConfig{
				DSN: dsn,
				Instances: map[string]*ClickHouseConfig{
					"analytics": {DSN: dsn},
				},
			},
		},
		{
			name: "named-only",
			cfg: &ClickHouseConfig{
				Instances: map[string]*ClickHouseConfig{
					"analytics": {DSN: dsn},
				},
			},
			want: "clickhouse instance analytics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ShutdownClickHouse()
			t.Cleanup(func() { ShutdownClickHouse() })

			db, err := InitClickHouse(tc.cfg)
			if err == nil {
				t.Fatalf("InitClickHouse() = %v, want a connection failure", db)
			}

			if db != nil {
				t.Fatalf("InitClickHouse() returned %v alongside the error, want nil", db)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitClickHouse() = %v, want it to name the failing instance", err)
			}

			if GetClickHouse() != nil {
				t.Fatal("GetClickHouse() = non-nil after a failed init")
			}

			if got := GetClickHouseInstances(); len(got) != 0 {
				t.Fatalf("GetClickHouseInstances() = %v, want none after a failed init", got)
			}
		})
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

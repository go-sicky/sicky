/**
 * @file nats_instances_test.go
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

func TestNATSConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &NATSConfig{
		URL: "nats://127.0.0.1:4222",
		Instances: map[string]*NATSConfig{
			"events": {},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrNATSURLEmpty) {
		t.Fatalf("Validate() = %v, want ErrNATSURLEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.events") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// Shape errors must be caught per instance and name the instance, so an
// operator does not have to guess which block is wrong.
func TestNATSConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &NATSConfig{
		URL: "nats://127.0.0.1:4222",
		Instances: map[string]*NATSConfig{
			"events": {
				URL:      "nats://127.0.0.1:4222",
				Token:    "token",
				Username: "user",
			},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrNATSAuthConflict) {
		t.Fatalf("Validate() = %v, want ErrNATSAuthConflict", err)
	}

	if !strings.Contains(err.Error(), "instances.events") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestNATSConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *NATSConfig
	}{
		{
			name: "instances",
			instance: &NATSConfig{
				URL:       "nats://127.0.0.1:4222",
				Instances: map[string]*NATSConfig{"nested": {URL: "nats://127.0.0.1:4223"}},
			},
		},
		{
			name:     "default_instance",
			instance: &NATSConfig{URL: "nats://127.0.0.1:4222", DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &NATSConfig{URL: "nats://127.0.0.1:4222", NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &NATSConfig{
				URL:       "nats://127.0.0.1:4222",
				Instances: map[string]*NATSConfig{"events": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// resolveInstances validates a copy: Ensure fills the reconnect defaults, and
// the caller's map is the caller's property.
func TestNATSConfigInstancesDoNotMutateCaller(t *testing.T) {
	instance := &NATSConfig{URL: "nats://127.0.0.1:4223"}
	cfg := &NATSConfig{Instances: map[string]*NATSConfig{"events": instance}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["events"].TimeoutSec; got != 0 {
		t.Fatalf("Validate() wrote %d into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "events" {
		t.Fatalf("resolveInstances() names = %v, want [events]", names)
	}

	if instances["events"] == cfg.Instances["events"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if got := instances["events"].TimeoutSec; got != DefaultNatsTimeoutSec {
		t.Fatalf("resolved instance TimeoutSec = %d, want %d", got, DefaultNatsTimeoutSec)
	}
}

func TestNATSValidateDefaultSelection(t *testing.T) {
	instances := map[string]*NATSConfig{
		"events": {URL: "nats://127.0.0.1:4222"},
		"jobs":   {URL: "nats://127.0.0.1:4223"},
	}

	if err := (&NATSConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&NATSConfig{DefaultInstance: "jobs", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&NATSConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&NATSConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&NATSConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&NATSConfig{NoDefault: true, DefaultInstance: "events", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

// InitNATS must fail fast and register nothing: an unreachable server is an
// ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitNATSFailFast(t *testing.T) {
	const url = "nats://127.0.0.1:1"

	for _, tc := range []struct {
		name string
		cfg  *NATSConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &NATSConfig{
				URL:              url,
				TimeoutSec:       1,
				ReconnectWaitSec: 1,
				MaxReconnects:    1,
				Instances: map[string]*NATSConfig{
					"events": {URL: url, TimeoutSec: 1, ReconnectWaitSec: 1, MaxReconnects: 1},
				},
			},
		},
		{
			name: "named-only",
			cfg: &NATSConfig{
				Instances: map[string]*NATSConfig{
					"events": {URL: url, TimeoutSec: 1, ReconnectWaitSec: 1, MaxReconnects: 1},
				},
			},
			want: "nats instance events",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = ShutdownNATS()
			t.Cleanup(func() { _ = ShutdownNATS() })

			conn, err := InitNATS(tc.cfg)
			if err == nil {
				t.Fatalf("InitNATS() = %v, want a connection failure", conn)
			}

			if conn != nil {
				t.Fatalf("InitNATS() returned %v alongside the error, want nil", conn)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitNATS() = %v, want it to name the failing instance", err)
			}

			if GetNATS() != nil {
				t.Fatal("GetNATS() = non-nil after a failed init")
			}

			if got := GetNATSInstances(); len(got) != 0 {
				t.Fatalf("GetNATSInstances() = %v, want none after a failed init", got)
			}
		})
	}
}

func TestInitNATSNoDefaultWithoutInstances(t *testing.T) {
	_ = ShutdownNATS()
	t.Cleanup(func() { _ = ShutdownNATS() })

	if _, err := InitNATS(&NATSConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitNATS() = %v, want ErrInstancesEmpty", err)
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

/**
 * @file mqtt_instances_test.go
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

func TestMQTTConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &MQTTConfig{
		Broker: "tcp://127.0.0.1:1883",
		Instances: map[string]*MQTTConfig{
			"fleet": {ClientID: "fleet-1"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrMQTTBrokerEmpty) {
		t.Fatalf("Validate() = %v, want ErrMQTTBrokerEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.fleet") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// Shape errors must be caught per instance and name the instance, so an
// operator does not have to guess which block is wrong.
func TestMQTTConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &MQTTConfig{
		Broker: "tcp://127.0.0.1:1883",
		Instances: map[string]*MQTTConfig{
			"fleet": {Broker: "tcp://127.0.0.1:1884", Password: "orphan"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrMQTTPasswordOrphan) {
		t.Fatalf("Validate() = %v, want ErrMQTTPasswordOrphan", err)
	}

	if !strings.Contains(err.Error(), "instances.fleet") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestMQTTConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *MQTTConfig
	}{
		{
			name: "instances",
			instance: &MQTTConfig{
				Broker:    "tcp://127.0.0.1:1883",
				Instances: map[string]*MQTTConfig{"nested": {Broker: "tcp://127.0.0.1:1884"}},
			},
		},
		{
			name:     "default_instance",
			instance: &MQTTConfig{Broker: "tcp://127.0.0.1:1883", DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &MQTTConfig{Broker: "tcp://127.0.0.1:1883", NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &MQTTConfig{
				Broker:    "tcp://127.0.0.1:1883",
				Instances: map[string]*MQTTConfig{"fleet": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// resolveInstances validates a copy: Ensure fills the timing defaults, and the
// caller's map is the caller's property.
func TestMQTTConfigInstancesDoNotMutateCaller(t *testing.T) {
	instance := &MQTTConfig{Broker: "tcp://127.0.0.1:1884"}
	cfg := &MQTTConfig{Instances: map[string]*MQTTConfig{"fleet": instance}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["fleet"].KeepAliveSec; got != 0 {
		t.Fatalf("Validate() wrote %d into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "fleet" {
		t.Fatalf("resolveInstances() names = %v, want [fleet]", names)
	}

	if instances["fleet"] == cfg.Instances["fleet"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if got := instances["fleet"].KeepAliveSec; got != DefaultMQTTKeepAliveSec {
		t.Fatalf("resolved instance KeepAliveSec = %d, want %d", got, DefaultMQTTKeepAliveSec)
	}
}

func TestMQTTValidateDefaultSelection(t *testing.T) {
	instances := map[string]*MQTTConfig{
		"fleet": {Broker: "tcp://127.0.0.1:1883"},
		"edge":  {Broker: "tcp://127.0.0.1:1884"},
	}

	if err := (&MQTTConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&MQTTConfig{DefaultInstance: "fleet", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&MQTTConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&MQTTConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&MQTTConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&MQTTConfig{NoDefault: true, DefaultInstance: "fleet", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

// InitMQTT must fail fast and register nothing: an unreachable broker is an
// ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitMQTTFailFast(t *testing.T) {
	const broker = "tcp://127.0.0.1:1"

	for _, tc := range []struct {
		name string
		cfg  *MQTTConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &MQTTConfig{
				Broker:            broker,
				ConnectTimeoutSec: 1,
				Instances: map[string]*MQTTConfig{
					"fleet": {Broker: broker, ConnectTimeoutSec: 1},
				},
			},
		},
		{
			name: "named-only",
			cfg: &MQTTConfig{
				Instances: map[string]*MQTTConfig{
					"fleet": {Broker: broker, ConnectTimeoutSec: 1},
				},
			},
			want: "mqtt instance fleet",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = ShutdownMQTT()
			t.Cleanup(func() { _ = ShutdownMQTT() })

			client, err := InitMQTT(tc.cfg)
			if err == nil {
				t.Fatalf("InitMQTT() = %v, want a connection failure", client)
			}

			if client != nil {
				t.Fatalf("InitMQTT() returned %v alongside the error, want nil", client)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitMQTT() = %v, want it to name the failing instance", err)
			}

			if GetMQTT() != nil {
				t.Fatal("GetMQTT() non-nil after a failed init")
			}

			if got := GetMQTTInstances(); len(got) != 0 {
				t.Fatalf("GetMQTTInstances() = %v, want none after a failed init", got)
			}
		})
	}
}

// A generated client_id must stay out of the caller's config: a repeated Init
// would otherwise share one id between every client it builds.
func TestInitMQTTDoesNotWriteGeneratedClientID(t *testing.T) {
	_ = ShutdownMQTT()
	t.Cleanup(func() { _ = ShutdownMQTT() })

	// The top-level config is passed to the builder as the caller's own
	// pointer, so this is the case that catches a missing copy.
	topLevel := &MQTTConfig{Broker: "tcp://127.0.0.1:1", ConnectTimeoutSec: 1}

	_, _ = InitMQTT(topLevel)

	if topLevel.ClientID != "" {
		t.Fatalf("InitMQTT() wrote %q into the caller's top-level config", topLevel.ClientID)
	}

	// Named instances are resolved through copies already, but the builder
	// must not depend on that.
	cfg := &MQTTConfig{
		Instances: map[string]*MQTTConfig{
			"fleet": {Broker: "tcp://127.0.0.1:1", ConnectTimeoutSec: 1},
		},
	}

	_, _ = InitMQTT(cfg)

	if got := cfg.Instances["fleet"].ClientID; got != "" {
		t.Fatalf("InitMQTT() wrote %q into the caller's instance config", got)
	}
}

// Two clients sharing a client_id fight over one broker session, so the
// conflict is a config error rather than a silent one-side-wins connection.
func TestInitMQTTRejectsDuplicateClientID(t *testing.T) {
	const broker = "tcp://127.0.0.1:1"

	duplicates := []struct {
		name string
		cfg  *MQTTConfig
	}{
		{
			name: "top-level and named",
			cfg: &MQTTConfig{
				Broker:            broker,
				ClientID:          "shared",
				ConnectTimeoutSec: 1,
				Instances: map[string]*MQTTConfig{
					"fleet": {Broker: broker, ClientID: "shared", ConnectTimeoutSec: 1},
				},
			},
		},
		{
			name: "two named instances",
			cfg: &MQTTConfig{
				Instances: map[string]*MQTTConfig{
					"alpha": {Broker: broker, ClientID: "shared", ConnectTimeoutSec: 1},
					"beta":  {Broker: broker, ClientID: "shared", ConnectTimeoutSec: 1},
				},
			},
		},
	}

	for _, tc := range duplicates {
		t.Run(tc.name, func(t *testing.T) {
			_ = ShutdownMQTT()
			t.Cleanup(func() { _ = ShutdownMQTT() })

			client, err := InitMQTT(tc.cfg)
			if !errors.Is(err, ErrMQTTClientIDDuplicate) {
				t.Fatalf("InitMQTT() = %v, want ErrMQTTClientIDDuplicate", err)
			}

			if client != nil {
				t.Fatalf("InitMQTT() returned %v alongside the error, want nil", client)
			}

			if !strings.Contains(err.Error(), "shared") {
				t.Fatalf("InitMQTT() = %v, want the duplicated client_id in the error", err)
			}

			if GetMQTT() != nil || len(GetMQTTInstances()) != 0 {
				t.Fatal("a duplicate client_id must not register anything")
			}
		})
	}
}

// The top-level config is not part of the client set when a named instance is
// the selected default, so its client_id cannot conflict with that instance.
func TestInitMQTTDuplicateCheckSkipsUnusedTopLevel(t *testing.T) {
	_ = ShutdownMQTT()
	t.Cleanup(func() { _ = ShutdownMQTT() })

	_, err := InitMQTT(&MQTTConfig{
		// Incomplete on purpose: default_instance must select the named
		// instance, which leaves the top-level block unused.
		ClientID:        "shared",
		DefaultInstance: "fleet",
		Instances: map[string]*MQTTConfig{
			"fleet": {Broker: "tcp://127.0.0.1:1", ClientID: "shared", ConnectTimeoutSec: 1},
		},
	})
	if err == nil {
		t.Fatal("InitMQTT() = nil, want the connection failure")
	}

	if errors.Is(err, ErrMQTTClientIDDuplicate) {
		t.Fatalf("InitMQTT() = %v, want the unused top-level id to be excluded", err)
	}
}

func TestInitMQTTEmptyClientIDsDoNotCollide(t *testing.T) {
	_ = ShutdownMQTT()
	t.Cleanup(func() { _ = ShutdownMQTT() })

	_, err := InitMQTT(&MQTTConfig{
		Broker:            "tcp://127.0.0.1:1",
		ConnectTimeoutSec: 1,
		Instances: map[string]*MQTTConfig{
			"fleet": {Broker: "tcp://127.0.0.1:1", ConnectTimeoutSec: 1},
		},
	})
	if err == nil {
		t.Fatal("InitMQTT() = nil, want the connection failure")
	}

	if errors.Is(err, ErrMQTTClientIDDuplicate) {
		t.Fatalf("InitMQTT() = %v, want no duplicate error for generated ids", err)
	}
}

func TestInitMQTTNoDefaultWithoutInstances(t *testing.T) {
	_ = ShutdownMQTT()
	t.Cleanup(func() { _ = ShutdownMQTT() })

	if _, err := InitMQTT(&MQTTConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitMQTT() = %v, want ErrInstancesEmpty", err)
	}
}

// A second Init keeps already-registered instances first-wins. Their
// client_id is not rebuilt, so it must not create a false duplicate with a
// new instance in the second config.
func TestMQTTClientIDValidationSkipsRegisteredInstances(t *testing.T) {
	ClearMQTTAll()
	t.Cleanup(func() { ClearMQTTAll() })

	mqttInstances.putFirst("cache", mqttInstance{})

	cfg := &MQTTConfig{
		Broker: "tcp://127.0.0.1:1",
		Instances: map[string]*MQTTConfig{
			"cache": {Broker: "tcp://127.0.0.1:1", ClientID: "same"},
			"other": {Broker: "tcp://127.0.0.1:1", ClientID: "same"},
		},
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances: %v", err)
	}

	if err := cfg.validateClientIDs(defaultSelection{}, instances, names); err != nil {
		t.Fatalf("validateClientIDs = %v, want nil when the duplicate name is already registered", err)
	}

	ClearMQTTAll()

	if err := cfg.validateClientIDs(defaultSelection{}, instances, names); !errors.Is(err, ErrMQTTClientIDDuplicate) {
		t.Fatalf("validateClientIDs = %v, want ErrMQTTClientIDDuplicate without the registered skip", err)
	}
}

// Registered clients keep their broker session; a new instance that reuses
// their effective id must be rejected even though the old one is not rebuilt.
func TestMQTTClientIDValidationSeedsRegisteredClientIDs(t *testing.T) {
	ClearMQTTAll()
	t.Cleanup(func() { ClearMQTTAll() })

	mqttInstances.putFirst("cache", mqttInstance{clientID: "same"})

	cfg := &MQTTConfig{
		Broker: "tcp://127.0.0.1:1",
		Instances: map[string]*MQTTConfig{
			"other": {Broker: "tcp://127.0.0.1:1", ClientID: "same"},
		},
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances: %v", err)
	}

	if err := cfg.validateClientIDs(defaultSelection{}, instances, names); !errors.Is(err, ErrMQTTClientIDDuplicate) {
		t.Fatalf("validateClientIDs = %v, want ErrMQTTClientIDDuplicate against the registered id", err)
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

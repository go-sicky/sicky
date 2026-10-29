/**
 * @file mongo_instances_test.go
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
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &MongoConfig{
		URI: "mongodb://127.0.0.1:27017",
		Instances: map[string]*MongoConfig{
			"reports": {DB: "reports"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrMongoURIEmpty) {
		t.Fatalf("Validate() = %v, want ErrMongoURIEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.reports") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestMongoConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *MongoConfig
	}{
		{
			name: "instances",
			instance: &MongoConfig{
				URI:       "mongodb://127.0.0.1:27017/reports",
				Instances: map[string]*MongoConfig{"nested": {URI: "mongodb://127.0.0.1:27017/nested"}},
			},
		},
		{
			name:     "default_instance",
			instance: &MongoConfig{URI: "mongodb://127.0.0.1:27017/reports", DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &MongoConfig{URI: "mongodb://127.0.0.1:27017/reports", NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &MongoConfig{
				URI:       "mongodb://127.0.0.1:27017",
				Instances: map[string]*MongoConfig{"reports": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// Shape errors are caught per instance and name the instance, so an operator
// does not have to guess which block is wrong.
func TestMongoConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &MongoConfig{
		URI: "mongodb://127.0.0.1:27017",
		Instances: map[string]*MongoConfig{
			"reports": {
				URI:               "mongodb://127.0.0.1:27017/reports",
				ConnectTimeoutSec: -1,
			},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrMongoOptionInvalid) {
		t.Fatalf("Validate() = %v, want ErrMongoOptionInvalid", err)
	}

	if !strings.Contains(err.Error(), "instances.reports") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// resolveInstances returns copies: the caller keeps ownership of its map, and
// the effective database of each copy is preserved for GetMongoDBOf.
func TestMongoConfigInstancesDoNotMutateCaller(t *testing.T) {
	cfg := &MongoConfig{
		URI: "mongodb://127.0.0.1:27017",
		Instances: map[string]*MongoConfig{
			"reports": {URI: "mongodb://127.0.0.1:27017/reports", DB: "reports"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "reports" {
		t.Fatalf("resolveInstances() names = %v, want [reports]", names)
	}

	if instances["reports"] == cfg.Instances["reports"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if got := effectiveMongoDB(instances["reports"]); got != "reports" {
		t.Fatalf("resolved instance database = %q, want reports", got)
	}

	if cfg.Instances["reports"].Instances != nil {
		t.Fatal("Validate() mutated the caller's instance config")
	}
}

func TestMongoValidateDefaultSelection(t *testing.T) {
	instances := map[string]*MongoConfig{
		"reports": {URI: "mongodb://127.0.0.1:27017/reports"},
		"audit":   {URI: "mongodb://127.0.0.1:27017/audit"},
	}

	if err := (&MongoConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&MongoConfig{DefaultInstance: "reports", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&MongoConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&MongoConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&MongoConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&MongoConfig{NoDefault: true, DefaultInstance: "reports", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

// GetMongoDBOf resolves the instance's stored database, lets an explicit name
// win, and returns nil for an unknown instance. The handle is never dialed:
// Database() only builds a deployment-bound handle.
func TestGetMongoDBOf(t *testing.T) {
	ShutdownMongo()
	t.Cleanup(func() { ClearMongoAll() })

	client, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		t.Fatalf("mongo.Connect() = %v, want nil (no reachability check)", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if derr := client.Disconnect(ctx); derr != nil {
			t.Errorf("Disconnect() = %v, want nil", derr)
		}
	})

	if _, stored := mongoInstances.putFirst("reports", mongoInstance{client: client, db: "reportsdb"}); !stored {
		t.Fatal("could not register the reports instance")
	}

	if db := GetMongoDBOf("reports"); db == nil || db.Name() != "reportsdb" {
		t.Fatalf("GetMongoDBOf(reports) = %v, want reportsdb", db)
	}

	if db := GetMongoDBOf("  Reports  "); db == nil || db.Name() != "reportsdb" {
		t.Fatalf("GetMongoDBOf(Reports) = %v, want the normalized instance", db)
	}

	if db := GetMongoDBOf("reports", "override"); db == nil || db.Name() != "override" {
		t.Fatalf("GetMongoDBOf(reports, override) = %v, want override", db)
	}

	if db := GetMongoDBOf("reports", "  "); db == nil || db.Name() != "reportsdb" {
		t.Fatalf("GetMongoDBOf(reports, blank) = %v, want the stored database", db)
	}

	if _, stored := mongoInstances.putFirst("nodb", mongoInstance{client: client}); !stored {
		t.Fatal("could not register the nodb instance")
	}

	if db := GetMongoDBOf("nodb"); db != nil {
		t.Fatalf("GetMongoDBOf(nodb) = %v, want nil without a resolvable database", db)
	}

	if db := GetMongoDBOf("missing"); db != nil {
		t.Fatalf("GetMongoDBOf(missing) = %v, want nil", db)
	}

	// An empty instance delegates to GetMongoDB, which is uninitialized here.
	if db := GetMongoDBOf("", "explicit"); db != nil {
		t.Fatalf("GetMongoDBOf(\"\") = %v, want the nil shared singleton", db)
	}
}

func TestInitMongoNoDefaultWithoutInstances(t *testing.T) {
	ShutdownMongo()
	t.Cleanup(func() { ShutdownMongo() })

	if _, err := InitMongo(&MongoConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitMongo() = %v, want ErrInstancesEmpty", err)
	}
}

// InitMongo must fail fast and register nothing: an unreachable server is an
// ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitMongoFailFast(t *testing.T) {
	const uri = "mongodb://127.0.0.1:1/?connectTimeoutMS=200&serverSelectionTimeoutMS=200"

	for _, tc := range []struct {
		name string
		cfg  *MongoConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &MongoConfig{
				URI:               uri,
				ConnectTimeoutSec: 1,
				Instances: map[string]*MongoConfig{
					"reports": {URI: uri, ConnectTimeoutSec: 1},
				},
			},
		},
		{
			name: "named-only",
			cfg: &MongoConfig{
				Instances: map[string]*MongoConfig{
					"reports": {URI: uri, ConnectTimeoutSec: 1},
				},
			},
			want: "mongo instance reports",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ShutdownMongo()
			t.Cleanup(func() { ShutdownMongo() })

			client, err := InitMongo(tc.cfg)
			if err == nil {
				t.Fatalf("InitMongo() = %v, want a connection failure", client)
			}

			if client != nil {
				t.Fatalf("InitMongo() returned %v alongside the error, want nil", client)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitMongo() = %v, want it to name the failing instance", err)
			}

			if GetMongo() != nil {
				t.Fatal("GetMongo() = non-nil after a failed init")
			}

			if got := GetMongoInstances(); len(got) != 0 {
				t.Fatalf("GetMongoInstances() = %v, want none after a failed init", got)
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

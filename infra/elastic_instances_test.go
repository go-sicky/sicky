/**
 * @file elastic_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestElasticConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &ElasticConfig{
		Addresses: []string{"http://127.0.0.1:9200"},
		Instances: map[string]*ElasticConfig{
			"search": {Username: "elastic"},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrElasticNoEndpoint) {
		t.Fatalf("Validate() = %v, want ErrElasticNoEndpoint for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.search") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestElasticConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *ElasticConfig
	}{
		{
			name: "instances",
			instance: &ElasticConfig{
				Addresses: []string{"http://127.0.0.1:9200"},
				Instances: map[string]*ElasticConfig{"nested": {Addresses: []string{"http://127.0.0.1:9201"}}},
			},
		},
		{
			name:     "default_instance",
			instance: &ElasticConfig{Addresses: []string{"http://127.0.0.1:9200"}, DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &ElasticConfig{Addresses: []string{"http://127.0.0.1:9200"}, NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ElasticConfig{
				Addresses: []string{"http://127.0.0.1:9200"},
				Instances: map[string]*ElasticConfig{"search": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// Shape errors are caught per instance and name the instance, so an operator
// does not have to guess which block is wrong.
func TestElasticConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &ElasticConfig{
		Addresses: []string{"http://127.0.0.1:9200"},
		Instances: map[string]*ElasticConfig{
			"search": {
				Addresses:  []string{"http://127.0.0.1:9201"},
				TimeoutSec: -1,
			},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrElasticTimeoutInvalid) {
		t.Fatalf("Validate() = %v, want ErrElasticTimeoutInvalid", err)
	}

	if !strings.Contains(err.Error(), "instances.search") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// An empty config keeps reporting ErrElasticNoEndpoint: there is no instance
// to fall back to, so the identity verdict still reaches the caller.
func TestElasticValidateKeepsEndpointErrorOrder(t *testing.T) {
	if err := (&ElasticConfig{}).Ensure().Validate(); !errors.Is(err, ErrElasticNoEndpoint) {
		t.Fatalf("empty config = %v, want ErrElasticNoEndpoint", err)
	}

	if err := (&ElasticConfig{Addresses: []string{"http://127.0.0.1:9200"}, TimeoutSec: -1}).Validate(); !errors.Is(err, ErrElasticTimeoutInvalid) {
		t.Fatalf("complete endpoint + negative timeout = %v, want ErrElasticTimeoutInvalid", err)
	}
}

// A complete top-level endpoint plus instances is allowed; the instances are
// still validated and resolvable.
func TestElasticValidateTopLevelAndInstances(t *testing.T) {
	cfg := &ElasticConfig{
		Addresses: []string{"http://127.0.0.1:9200"},
		Instances: map[string]*ElasticConfig{
			"search": {Addresses: []string{"http://127.0.0.1:9201"}},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	selection, err := resolveDefaultSelection(
		cfg.NoDefault,
		cfg.DefaultInstance,
		[]string{"search"},
		cfg.validateIdentity() == nil,
		cfg.validateIdentity(),
	)
	if err != nil {
		t.Fatalf("resolveDefaultSelection() = %v, want nil", err)
	}

	if !selection.UseTopLevel {
		t.Fatalf("selection = %+v, want the complete top-level config", selection)
	}
}

// resolveInstances returns copies: Ensure fills the copy's timeout default
// without writing it back into the caller's map.
func TestElasticConfigInstancesDoNotMutateCaller(t *testing.T) {
	cfg := &ElasticConfig{
		Addresses: []string{"http://127.0.0.1:9200"},
		Instances: map[string]*ElasticConfig{
			"search": {Addresses: []string{"http://127.0.0.1:9201"}},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["search"].TimeoutSec; got != 0 {
		t.Fatalf("Validate() wrote %d into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "search" {
		t.Fatalf("resolveInstances() names = %v, want [search]", names)
	}

	if instances["search"] == cfg.Instances["search"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if got := instances["search"].TimeoutSec; got != DefaultElasticTimeoutSec {
		t.Fatalf("resolved instance TimeoutSec = %d, want %d", got, DefaultElasticTimeoutSec)
	}
}

func TestElasticValidateDefaultSelection(t *testing.T) {
	instances := map[string]*ElasticConfig{
		"search": {Addresses: []string{"http://127.0.0.1:9200"}},
		"logs":   {Addresses: []string{"http://127.0.0.1:9201"}},
	}

	if err := (&ElasticConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&ElasticConfig{DefaultInstance: "search", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&ElasticConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&ElasticConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&ElasticConfig{NoDefault: true, Addresses: []string{"http://127.0.0.1:9200"}}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&ElasticConfig{NoDefault: true, DefaultInstance: "search", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

func TestInitElasticNoDefaultWithoutInstances(t *testing.T) {
	ShutdownElastic()
	t.Cleanup(func() { ShutdownElastic() })

	if _, err := InitElastic(&ElasticConfig{NoDefault: true}); !errors.Is(err, ErrInstancesEmpty) {
		t.Fatalf("InitElastic() = %v, want ErrInstancesEmpty", err)
	}
}

// InitElastic must fail fast and register nothing: an unreachable cluster is
// an ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitElasticFailFast(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *ElasticConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &ElasticConfig{
				Addresses:  []string{"http://127.0.0.1:1"},
				TimeoutSec: 1,
				Instances: map[string]*ElasticConfig{
					"search": {Addresses: []string{"http://127.0.0.1:1"}, TimeoutSec: 1},
				},
			},
		},
		{
			name: "named-only",
			cfg: &ElasticConfig{
				Instances: map[string]*ElasticConfig{
					"search": {Addresses: []string{"http://127.0.0.1:1"}, TimeoutSec: 1},
				},
			},
			want: "elastic instance search",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ShutdownElastic()
			t.Cleanup(func() { ShutdownElastic() })

			client, err := InitElastic(tc.cfg)
			if err == nil {
				t.Fatalf("InitElastic() = %v, want a connection failure", client)
			}

			if client != nil {
				t.Fatalf("InitElastic() returned %v alongside the error, want nil", client)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitElastic() = %v, want it to name the failing instance", err)
			}

			if GetElastic() != nil {
				t.Fatal("GetElastic() = non-nil after a failed init")
			}

			if got := GetElasticInstances(); len(got) != 0 {
				t.Fatalf("GetElasticInstances() = %v, want none after a failed init", got)
			}
		})
	}
}

// A named build that fails after the top-level client (and an earlier sorted
// instance) succeeded must leave nothing behind: the half-installed component
// would advertise a default that no later init could replace.
func TestInitElasticRollsBackBuiltClientsOnInstanceFailure(t *testing.T) {
	ShutdownElastic()
	t.Cleanup(func() { ShutdownElastic() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"fake","cluster_name":"fake","version":{"number":"9.0.0"}}`))
	}))
	defer srv.Close()

	_, err := InitElastic(&ElasticConfig{
		Addresses:  []string{srv.URL},
		TimeoutSec: 1,
		Instances: map[string]*ElasticConfig{
			// "alpha" sorts before "broken", so a client is built,
			// counted as up, and then rolled back.
			"alpha":  {Addresses: []string{srv.URL}, TimeoutSec: 1},
			"broken": {Addresses: []string{"http://127.0.0.1:1"}, TimeoutSec: 1},
		},
	})
	if err == nil {
		t.Fatal("InitElastic() = nil, want the instance build failure")
	}

	if !strings.Contains(err.Error(), "elastic instance broken") {
		t.Fatalf("InitElastic() = %v, want the failing instance named", err)
	}

	if GetElastic() != nil {
		t.Fatal("the top-level client must be rolled back when a named build fails")
	}

	if got := GetElasticInstances(); len(got) != 0 {
		t.Fatalf("GetElasticInstances() = %v, want none after a rolled-back init", got)
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

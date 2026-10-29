/**
 * @file ristretto_instances_test.go
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

func TestRistrettoConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &RistrettoConfig{
		NumCounters: 1000,
		MaxCost:     1000,
		Instances: map[string]*RistrettoConfig{
			"cache": {NumCounters: -1},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrRistrettoNegativeNumCounters) {
		t.Fatalf("Validate() = %v, want ErrRistrettoNegativeNumCounters", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.cache") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestRistrettoConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *RistrettoConfig
	}{
		{
			name: "instances",
			instance: &RistrettoConfig{
				Instances: map[string]*RistrettoConfig{"nested": {}},
			},
		},
		{
			name:     "default_instance",
			instance: &RistrettoConfig{DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &RistrettoConfig{NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &RistrettoConfig{
				Instances: map[string]*RistrettoConfig{"cache": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// resolveInstances validates a copy: Ensure fills the default sizing without
// writing it into the caller's map.
func TestRistrettoConfigInstancesDoNotMutateCaller(t *testing.T) {
	instance := &RistrettoConfig{}
	cfg := &RistrettoConfig{Instances: map[string]*RistrettoConfig{"cache": instance}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["cache"].NumCounters; got != 0 {
		t.Fatalf("Validate() wrote %d into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "cache" {
		t.Fatalf("resolveInstances() names = %v, want [cache]", names)
	}

	if instances["cache"] == cfg.Instances["cache"] {
		t.Fatal("resolveInstances must return copies, not the caller's configs")
	}

	if got := instances["cache"].NumCounters; got != DefaultRistrettoNumCounters {
		t.Fatalf("resolved instance NumCounters = %d, want %d", got, DefaultRistrettoNumCounters)
	}

	if got := instances["cache"].MaxCost; got != DefaultRistrettoMaxCost {
		t.Fatalf("resolved instance MaxCost = %d, want %d", got, DefaultRistrettoMaxCost)
	}
}

// A ristretto block has no required identity, so the top-level config is
// always complete: default_instance is ignored rather than resolved, and only
// no_default changes the selection.
func TestRistrettoValidateDefaultSelection(t *testing.T) {
	instances := map[string]*RistrettoConfig{
		"cache": {NumCounters: 1000, MaxCost: 1000},
		"queue": {NumCounters: 1000, MaxCost: 1000},
	}

	if err := (&RistrettoConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil", err)
	}

	if err := (&RistrettoConfig{DefaultInstance: "cache", Instances: instances}).Validate(); err != nil {
		t.Errorf("default_instance = %v, want nil (it is ignored)", err)
	}

	// Ignored means not resolved: an unknown name is not an error.
	if err := (&RistrettoConfig{DefaultInstance: "missing", Instances: instances}).Validate(); err != nil {
		t.Errorf("unknown default_instance = %v, want nil (it is ignored)", err)
	}

	if err := (&RistrettoConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&RistrettoConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&RistrettoConfig{NoDefault: true, DefaultInstance: "cache", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

func TestInitRistrettoNamedInstances(t *testing.T) {
	ClearRistrettoAll()
	t.Cleanup(func() { _ = ShutdownRistretto() })

	cache, err := InitRistretto(&RistrettoConfig{
		NumCounters: 1000,
		MaxCost:     1000,
		BufferItems: 64,
		Instances: map[string]*RistrettoConfig{
			"cache": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
			"queue": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
		},
	})
	if err != nil {
		t.Fatalf("InitRistretto() = %v, want nil", err)
	}

	if cache == nil || GetRistretto() != cache {
		t.Fatal("GetRistretto() must return the shared cache InitRistretto returned")
	}

	if GetRistretto("cache") == nil || GetRistretto("queue") == nil {
		t.Fatal("GetRistretto(name) = nil, want the named caches")
	}

	if GetRistretto("cache") == cache {
		t.Fatal("the named caches must be distinct from the top-level default")
	}

	if got := GetRistrettoInstances(); len(got) != 2 {
		t.Fatalf("GetRistrettoInstances() = %d entries, want 2", len(got))
	}

	namedCache := GetRistretto("cache")
	ClearRistretto("cache")
	if GetRistretto("cache") != nil {
		t.Fatal("ClearRistretto(cache) left the instance registered")
	}

	if GetRistretto() == nil {
		t.Fatal("ClearRistretto(cache) must not clear the default")
	}

	namedCache.Close()

	defaultCache := GetRistretto()
	queue := GetRistretto("queue")
	ClearRistrettoAll()
	defaultCache.Close()
	queue.Close()

	if GetRistretto() != nil || len(GetRistrettoInstances()) != 0 {
		t.Fatal("ClearRistrettoAll must clear the default and every named cache")
	}
}

func TestInitRistrettoNoDefault(t *testing.T) {
	ClearRistrettoAll()
	t.Cleanup(func() { _ = ShutdownRistretto() })

	cache, err := InitRistretto(&RistrettoConfig{
		NoDefault: true,
		Instances: map[string]*RistrettoConfig{
			"cache": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
		},
	})
	if err != nil {
		t.Fatalf("InitRistretto() = %v, want nil", err)
	}

	if cache != nil {
		t.Fatalf("InitRistretto() = %v, want nil with no_default", cache)
	}

	if GetRistretto() != nil {
		t.Fatal("GetRistretto() must be nil with no_default")
	}

	if GetRistretto("cache") == nil {
		t.Fatal("GetRistretto(cache) = nil, want the named cache")
	}

	if err := ShutdownRistretto(); err != nil {
		t.Fatalf("ShutdownRistretto() = %v, want nil", err)
	}

	if GetRistretto("cache") != nil {
		t.Fatal("ShutdownRistretto must clear the named caches")
	}
}

// The top-level config always wins for ristretto; default_instance selects
// nothing, so the named cache must not become an alias of the shared one.
func TestInitRistrettoDefaultInstanceIgnored(t *testing.T) {
	ClearRistrettoAll()
	t.Cleanup(func() { _ = ShutdownRistretto() })

	cache, err := InitRistretto(&RistrettoConfig{
		NumCounters:     1000,
		MaxCost:         1000,
		BufferItems:     64,
		DefaultInstance: "cache",
		Instances: map[string]*RistrettoConfig{
			"cache": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
		},
	})
	if err != nil {
		t.Fatalf("InitRistretto() = %v, want nil", err)
	}

	if cache == nil || GetRistretto() != cache {
		t.Fatal("GetRistretto() must be the top-level cache")
	}

	if GetRistretto("cache") == nil || GetRistretto("cache") == cache {
		t.Fatal("the named cache must exist separately from the top-level cache")
	}

	mu.RLock()
	alias := ristrettoDefaultInstance
	mu.RUnlock()

	if alias != "" {
		t.Fatalf("ristrettoDefaultInstance = %q, want empty (no alias)", alias)
	}

	ClearRistretto()
	if GetRistretto() != nil {
		t.Fatal("ClearRistretto() must clear the top-level cache")
	}

	if GetRistretto("cache") == nil {
		t.Fatal("clearing the top-level cache must not touch the named cache")
	}

	cache.Close()
}

func TestInitRistrettoTotalShutdown(t *testing.T) {
	ClearRistrettoAll()
	t.Cleanup(func() { _ = ShutdownRistretto() })

	if _, err := InitRistretto(&RistrettoConfig{
		NumCounters: 1000,
		MaxCost:     1000,
		BufferItems: 64,
		Instances: map[string]*RistrettoConfig{
			"cache": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
		},
	}); err != nil {
		t.Fatalf("InitRistretto() = %v, want nil", err)
	}

	if err := ShutdownRistretto(); err != nil {
		t.Fatalf("ShutdownRistretto() = %v, want nil", err)
	}

	if GetRistretto() != nil || len(GetRistrettoInstances()) != 0 {
		t.Fatal("ShutdownRistretto must clear the default and every named cache")
	}

	// A second shutdown is a no-op, not an error.
	if err := ShutdownRistretto(); err != nil {
		t.Fatalf("second ShutdownRistretto() = %v, want nil", err)
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

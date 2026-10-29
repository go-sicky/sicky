/**
 * @file badger_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func TestBadgerConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &BadgerConfig{
		Path: t.TempDir(),
		Instances: map[string]*BadgerConfig{
			"cache": {},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrBadgerPathEmpty) {
		t.Fatalf("Validate() = %v, want ErrBadgerPathEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.cache") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestBadgerConfigRejectsNestedInstances(t *testing.T) {
	for _, tc := range []struct {
		name     string
		instance *BadgerConfig
	}{
		{
			name: "instances",
			instance: &BadgerConfig{
				Path:      "/tmp/sicky-badger-nested",
				Instances: map[string]*BadgerConfig{"nested": {Path: "/tmp/sicky-badger-nested-deeper"}},
			},
		},
		{
			name:     "default_instance",
			instance: &BadgerConfig{Path: "/tmp/sicky-badger-nested", DefaultInstance: "nested"},
		},
		{
			name:     "no_default",
			instance: &BadgerConfig{Path: "/tmp/sicky-badger-nested", NoDefault: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &BadgerConfig{
				Path:      t.TempDir(),
				Instances: map[string]*BadgerConfig{"cache": tc.instance},
			}

			if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
				t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
			}
		})
	}
}

// resolveInstances returns copies: the caller keeps ownership of its map.
func TestBadgerConfigInstancesDoNotMutateCaller(t *testing.T) {
	instance := &BadgerConfig{Path: "/tmp/sicky-badger-caller"}
	cfg := &BadgerConfig{Instances: map[string]*BadgerConfig{"cache": instance}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if cfg.Instances["cache"] != instance {
		t.Fatal("Validate() replaced the caller's instance config")
	}

	if cfg.Instances["cache"].Instances != nil {
		t.Fatal("Validate() mutated the caller's instance config")
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
}

func TestBadgerValidateDefaultSelection(t *testing.T) {
	instances := map[string]*BadgerConfig{
		"cache": {Path: "/tmp/sicky-badger-cache"},
		"logs":  {Path: "/tmp/sicky-badger-logs"},
	}

	if err := (&BadgerConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&BadgerConfig{DefaultInstance: "logs", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&BadgerConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&BadgerConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&BadgerConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&BadgerConfig{NoDefault: true, DefaultInstance: "cache", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

func TestInitBadgerNamedInstances(t *testing.T) {
	_ = ShutdownBadger()
	t.Cleanup(func() { _ = ShutdownBadger() })

	db, err := InitBadger(&BadgerConfig{
		Path: t.TempDir(),
		Instances: map[string]*BadgerConfig{
			"cache": {Path: t.TempDir()},
			"logs":  {Path: t.TempDir()},
		},
	})
	if err != nil {
		t.Fatalf("InitBadger() = %v, want nil", err)
	}

	if db == nil || GetBadger() != db {
		t.Fatal("GetBadger() must return the shared storage InitBadger returned")
	}

	// The top-level path is complete, so it is the default even though named
	// instances exist.
	if GetBadger("cache") == nil || GetBadger("logs") == nil {
		t.Fatal("GetBadger(name) = nil, want the named storages")
	}

	if GetBadger("cache") == db {
		t.Fatal("the named instances must be distinct from the top-level default")
	}

	if got := GetBadgerInstances(); len(got) != 2 {
		t.Fatalf("GetBadgerInstances() = %d entries, want 2", len(got))
	}

	cacheDB := GetBadger("cache")
	ClearBadger("cache")
	if GetBadger("cache") != nil {
		t.Fatal("ClearBadger(cache) left the instance registered")
	}

	if GetBadger() == nil {
		t.Fatal("ClearBadger(cache) must not clear the default")
	}

	cacheDB.Close()

	defaultDB := GetBadger()
	logsDB := GetBadger("logs")
	ClearBadgerAll()
	defaultDB.Close()
	logsDB.Close()
	if GetBadger() != nil || len(GetBadgerInstances()) != 0 {
		t.Fatal("ClearBadgerAll must clear the default and every named instance")
	}

	if err := ShutdownBadger(); err != nil {
		t.Fatalf("ShutdownBadger() = %v, want nil", err)
	}
}

func TestInitBadgerDefaultInstanceAlias(t *testing.T) {
	_ = ShutdownBadger()
	t.Cleanup(func() { _ = ShutdownBadger() })

	if _, err := InitBadger(&BadgerConfig{
		DefaultInstance: "cache",
		Instances: map[string]*BadgerConfig{
			"cache": {Path: t.TempDir()},
			"logs":  {Path: t.TempDir()},
		},
	}); err != nil {
		t.Fatalf("InitBadger() = %v, want nil", err)
	}

	if GetBadger() == nil || GetBadger() != GetBadger("cache") {
		t.Fatal("GetBadger() must be the cache alias")
	}

	if GetBadgerInstances()["cache"] != GetBadger() {
		t.Fatal("the named snapshot must expose the same alias storage")
	}

	cacheDB := GetBadger("cache")
	ClearBadger("cache")
	if GetBadger() != nil {
		t.Fatal("clearing the default alias must clear the shared reference")
	}

	cacheDB.Close()

	if GetBadger("logs") == nil {
		t.Fatal("clearing cache must not clear logs")
	}
}

func TestInitBadgerNoDefault(t *testing.T) {
	_ = ShutdownBadger()
	t.Cleanup(func() { _ = ShutdownBadger() })

	db, err := InitBadger(&BadgerConfig{
		NoDefault: true,
		Instances: map[string]*BadgerConfig{
			"cache": {Path: t.TempDir()},
		},
	})
	if err != nil {
		t.Fatalf("InitBadger() = %v, want nil", err)
	}

	if db != nil {
		t.Fatalf("InitBadger() = %v, want nil with no_default", db)
	}

	if GetBadger() != nil {
		t.Fatal("GetBadger() must be nil with no_default")
	}

	if GetBadger("cache") == nil {
		t.Fatal("GetBadger(cache) = nil, want the named storage")
	}
}

func TestInitBadgerTotalShutdown(t *testing.T) {
	_ = ShutdownBadger()
	t.Cleanup(func() { _ = ShutdownBadger() })

	if _, err := InitBadger(&BadgerConfig{
		Path: t.TempDir(),
		Instances: map[string]*BadgerConfig{
			"cache": {Path: t.TempDir()},
		},
	}); err != nil {
		t.Fatalf("InitBadger() = %v, want nil", err)
	}

	if err := ShutdownBadger(); err != nil {
		t.Fatalf("ShutdownBadger() = %v, want nil", err)
	}

	if GetBadger() != nil || len(GetBadgerInstances()) != 0 {
		t.Fatal("ShutdownBadger must clear the default and every named instance")
	}

	// A second shutdown is a no-op, not an error.
	if err := ShutdownBadger(); err != nil {
		t.Fatalf("second ShutdownBadger() = %v, want nil", err)
	}
}

// A duplicate directory is caught with a clear config error instead of
// Badger's own file-lock failure, and nothing is left open behind it.
func TestInitBadgerRejectsDuplicatePath(t *testing.T) {
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		cfg  *BadgerConfig
	}{
		{
			name: "top-level and named",
			cfg: &BadgerConfig{
				Path: dir,
				Instances: map[string]*BadgerConfig{
					"cache": {Path: dir},
				},
			},
		},
		{
			name: "two named, different spelling",
			cfg: &BadgerConfig{
				Instances: map[string]*BadgerConfig{
					"cache": {Path: dir},
					"logs":  {Path: dir + string(filepath.Separator) + "."},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = ShutdownBadger()
			t.Cleanup(func() { _ = ShutdownBadger() })

			db, err := InitBadger(tc.cfg)
			if !errors.Is(err, ErrBadgerPathDuplicate) {
				t.Fatalf("InitBadger() = %v, want ErrBadgerPathDuplicate", err)
			}

			if db != nil {
				t.Fatalf("InitBadger() returned %v alongside the error, want nil", db)
			}

			if GetBadger() != nil || len(GetBadgerInstances()) != 0 {
				t.Fatal("a duplicate path must not register anything")
			}

			// The directory must not be left locked: routing the collision
			// through config validation has to happen before Badger opens
			// anything.
			probe, probeErr := badger.Open(badger.DefaultOptions(dir))
			if probeErr != nil {
				t.Fatalf("the directory is still locked after the rejection: %v", probeErr)
			}

			if cerr := probe.Close(); cerr != nil {
				t.Fatalf("probe close: %v", cerr)
			}
		})
	}
}

// A failing named build must close the top-level storage this call already
// opened; a half-installed component would advertise a default that no later
// first-wins init can replace.
func TestInitBadgerRollsBackTopLevelOnInstanceFailure(t *testing.T) {
	_ = ShutdownBadger()
	t.Cleanup(func() { _ = ShutdownBadger() })

	broken := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(broken, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	topLevelDir := t.TempDir()
	alphaDir := t.TempDir()

	_, err := InitBadger(&BadgerConfig{
		Path: topLevelDir,
		Instances: map[string]*BadgerConfig{
			// "alpha" sorts before "broken", so its storage is opened and
			// must be rolled back together with the top-level one.
			"alpha":  {Path: alphaDir},
			"broken": {Path: broken},
		},
	})
	if err == nil {
		t.Fatal("InitBadger() = nil, want the instance open failure")
	}

	if !strings.Contains(err.Error(), "badger instance broken") {
		t.Fatalf("InitBadger() = %v, want the failing instance named", err)
	}

	if GetBadger() != nil {
		t.Fatal("the top-level storage must be rolled back when a named build fails")
	}

	if got := GetBadgerInstances(); len(got) != 0 {
		t.Fatalf("GetBadgerInstances() = %v, want none after a rolled-back init", got)
	}

	// A rolled-back storage must also be closed: Badger locks its directory,
	// so an open handle would make the directory unusable even though it was
	// never registered.
	for _, dir := range []string{topLevelDir, alphaDir} {
		probe, probeErr := badger.Open(badger.DefaultOptions(dir))
		if probeErr != nil {
			t.Fatalf("rollback left %s locked: %v", dir, probeErr)
		}

		if cerr := probe.Close(); cerr != nil {
			t.Fatalf("probe close: %v", cerr)
		}
	}
}

// A second Init keeps already-registered instances first-wins. Their paths are
// not reopened, so they must not create a false duplicate with a new instance
// in the second config.
func TestBadgerPathValidationSkipsRegisteredInstances(t *testing.T) {
	ClearBadgerAll()
	t.Cleanup(func() { ClearBadgerAll() })

	badgerInstances.putFirst("cache", badgerInstance{})

	cfg := &BadgerConfig{
		Instances: map[string]*BadgerConfig{
			"cache": {Path: "/tmp/sicky-badger-same"},
			"other": {Path: "/tmp/sicky-badger-same"},
		},
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances: %v", err)
	}

	if err := cfg.validatePaths(defaultSelection{}, instances, names); err != nil {
		t.Fatalf("validatePaths = %v, want nil when the duplicate name is already registered", err)
	}

	ClearBadgerAll()

	if err := cfg.validatePaths(defaultSelection{}, instances, names); !errors.Is(err, ErrBadgerPathDuplicate) {
		t.Fatalf("validatePaths = %v, want ErrBadgerPathDuplicate without the registered skip", err)
	}
}

// Registered storages keep their directory lock; a new instance that reuses
// their path must be rejected even though the old one is not rebuilt.
func TestBadgerPathValidationSeedsRegisteredPaths(t *testing.T) {
	ClearBadgerAll()
	t.Cleanup(func() { ClearBadgerAll() })

	path := "/tmp/sicky-badger-same"
	badgerInstances.putFirst("cache", badgerInstance{path: resolveBadgerPath(path)})

	cfg := &BadgerConfig{
		Instances: map[string]*BadgerConfig{
			"other": {Path: path},
		},
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances: %v", err)
	}

	if err := cfg.validatePaths(defaultSelection{}, instances, names); !errors.Is(err, ErrBadgerPathDuplicate) {
		t.Fatalf("validatePaths = %v, want ErrBadgerPathDuplicate against the registered path", err)
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

/**
 * @file configschema_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// viper drops a key it cannot map, silently. So `{"sicky":{"infra":{"bu":
// {"dsn": ...}}}}` starts a service with no database and reports nothing, and
// `auth_tokn` leaves /config and /services unguarded with no complaint. The
// operator's only clue is a service that is not doing what its own config file
// says it should.
func TestConfigSchemaFlagsMistypedKeys(t *testing.T) {
	v := viper.New()
	v.SetConfigType("json")

	if err := v.ReadConfig(strings.NewReader(`{
		"sicky": {
			"log_level": "info",
			"manager": {"address": "127.0.0.1:8888", "auth_tokn": "typo"},
			"infra": {"bu": {"dsn": "postgres://x/y"}, "redis": {"addr": "127.0.0.1:6379"}},
			"tracer": {"type": "none"}
		},
		"server": {"http": {"adresss": ":8080"}}
	}`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	schema := buildConfigSchema(&nestedAppConfig{})

	got := schema.unknownKeys(v.AllKeys())

	for _, want := range []string{"sicky.infra.bu", "sicky.manager.auth_tokn"} {
		if !slices.Contains(got, want) {
			t.Errorf("%q must be reported as unknown; got %v", want, got)
		}
	}

	// The application's own block is not the framework's to judge: it has no
	// idea what the application put there, so `server.http.adress` must stay
	// silent. Warning about it would be guessing, and a check that guesses is
	// a check operators learn to ignore.
	if slices.Contains(got, "server.http.adresss") {
		t.Errorf("the framework must not report keys it does not own; got %v", got)
	}

	// Every well-formed key must be recognized, or the check is worse than
	// useless — it would bury a real typo under a wall of false warnings.
	for _, key := range []string{
		"sicky.log_level",
		"sicky.manager",
		"sicky.manager.address",
		"sicky.infra",
		"sicky.infra.redis",
		"sicky.infra.redis.addr",
		"sicky.tracer.type",
	} {
		if slices.Contains(got, key) {
			t.Errorf("%q is a valid key and must not be reported; got %v", key, got)
		}
	}
}

// The report is sorted, so two runs of the same configuration warn in the
// same order and a log diff is meaningful.
func TestConfigSchemaUnknownKeysAreSorted(t *testing.T) {
	schema := buildConfigSchema(&nestedAppConfig{})

	got := schema.unknownKeys([]string{
		"sicky.zeta", "sicky.alpha", "sicky.mid", "sicky.alpha",
	})

	want := []string{"sicky.alpha", "sicky.mid", "sicky.zeta"}
	if !slices.Equal(got, want) {
		t.Errorf("unknown keys = %v, want %v (sorted, deduplicated)", got, want)
	}
}

// A target with no Config at all must produce no prefixes that claim
// ownership, or the framework would report the application's entire
// configuration as unknown.
func TestConfigSchemaOwnsNothingWithoutAConfig(t *testing.T) {
	schema := buildConfigSchema(&struct {
		Whatever map[string]string `mapstructure:"whatever"`
	}{})

	if got := schema.unknownKeys([]string{"whatever.a", "server.http.address"}); len(got) != 0 {
		t.Errorf("a target with no Config must report nothing, got %v", got)
	}
}

// warns rather than fails: a typo must not stop a service from starting, and
// the warning must name the key.
func TestWarnUnknownConfigKeysDoesNotFailUnmarshal(t *testing.T) {
	v := viper.New()
	v.SetEnvPrefix(DefaultEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigType("json")

	if err := v.ReadConfig(strings.NewReader(
		`{"sicky":{"infra":{"bun":{"dsn":"postgres://x/y"},"bu":{"dsn":"typo"}}}}`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg := &nestedAppConfig{Sicky: &Config{}}

	// The real target for the check, without touching the package's viper.
	schema := buildConfigSchema(cfg)
	if got := schema.unknownKeys(v.AllKeys()); !slices.Contains(got, "sicky.infra.bu") {
		t.Fatalf("schema did not flag the typo: %v", got)
	}

	// And unmarshaling still succeeds despite the typo in the file.
	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal must not fail on an unknown key: %v", err)
	}

	if cfg.Sicky.Infra == nil || cfg.Sicky.Infra.Bun == nil {
		t.Fatal("the well-formed key must still be applied")
	}

	if cfg.Sicky.Infra.Bun.DSN != "postgres://x/y" {
		t.Errorf("bun dsn = %q; a typo in a sibling key must not disturb a valid one", cfg.Sicky.Infra.Bun.DSN)
	}
}

// Every key the current sensitiveEnvKeys list names must be a real field, or
// binding it does nothing — the phantom-entry problem TestSensitiveEnvKeysHave
// NoPhantomEntries guards by hand for two keys. This guards all of them by
// walking the schema.
func TestSensitiveEnvKeysAllNameRealFields(t *testing.T) {
	schema := buildConfigSchema(&Config{})

	for _, key := range sensitiveEnvKeys {
		if _, ok := schema.nodes[key]; !ok {
			t.Errorf("sensitiveEnvKeys names %q but no Config field maps to it: "+
				"binding it is a no-op and makes the ignored-variable check call it known", key)
		}
	}
}

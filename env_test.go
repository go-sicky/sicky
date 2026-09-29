/*
 * Copyright (c) 2026, The go-sicky Authors
 * SPDX-License-Identifier: MIT
 *
 * @file    env_test.go
 * @package sicky
 * @author  Dr.NP <np@herewe.tech>
 * @since   09/29/2026
 */

package sicky

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// TestSensitiveEnvKeysCoverRegistryCredentials pins the C7 fix:
// registry.redis.password is a credential in exactly the same class as
// infra.redis.password (which was already bound), so leaving it out made
// SICKY_REGISTRY_REDIS_PASSWORD resolve via AutomaticEnv but be dropped by
// Unmarshal whenever the config file's registry.redis block omits
// `password` - the registry then connects with whatever the file says,
// usually nothing.
func TestSensitiveEnvKeysCoverRegistryCredentials(t *testing.T) {
	for _, key := range []string{
		"registry.redis.password",
		"registry.local.registry_file_path",
		"infra.mqtt.ca_file",
		"infra.nats.creds_file",
		"infra.nats.nkey_file",
		"infra.nats.root_ca_file",
		"infra.elastic.ca_cert_file",
	} {
		if !slices.Contains(sensitiveEnvKeys, key) {
			t.Errorf("%s must be in sensitiveEnvKeys: viper's AutomaticEnv is invisible to Unmarshal for a key absent from the config file", key)
		}
	}
}

// TestSensitiveEnvKeysHaveNoPhantomEntries is the other half. "registry.type"
// and "broker.type" used to be listed, but neither registry.Config nor
// broker.Config declares a Type field. Binding a non-existent key is a
// no-op, and it also made ignoredEnv treat SICKY_REGISTRY_TYPE as *known*,
// so the operator got neither an applied override nor the "will be
// ignored" warning - the variable was accepted and silently dropped.
func TestSensitiveEnvKeysHaveNoPhantomEntries(t *testing.T) {
	for _, key := range []string{"registry.type", "broker.type"} {
		if slices.Contains(sensitiveEnvKeys, key) {
			t.Errorf("%s is in sensitiveEnvKeys but no Config declares that field: binding it does nothing and suppresses the ignored-variable warning", key)
		}
	}
}

// TestBindSensitiveEnvResolvesRegistryPassword is the behavioral half: it
// proves the binding actually makes the value survive Unmarshal, which is
// the whole point of the list.
func TestBindSensitiveEnvResolvesRegistryPassword(t *testing.T) {
	t.Setenv("SICKY_REGISTRY_REDIS_PASSWORD", "s3cret")

	v := viper.New()
	v.SetEnvPrefix(DefaultEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// The config file deliberately omits the key, which is exactly the
	// situation the list exists for.
	v.SetConfigType("json")

	if err := v.ReadConfig(strings.NewReader(`{"registry":{"redis":{"addr":"127.0.0.1:6379"}}}`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	bindSensitiveEnv(v)

	var cfg struct {
		Registry struct {
			Redis struct {
				Addr     string `mapstructure:"addr"`
				Password string `mapstructure:"password"`
			} `mapstructure:"redis"`
		} `mapstructure:"registry"`
	}

	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Registry.Redis.Password != "s3cret" {
		t.Errorf("registry.redis.password = %q, want %q: the env override was dropped by Unmarshal",
			cfg.Registry.Redis.Password, "s3cret")
	}

	if got := cfg.Registry.Redis.Addr; got != "127.0.0.1:6379" {
		t.Errorf("registry.redis.addr = %q, want %q: the env binding must not clobber file-backed keys", got, "127.0.0.1:6379")
	}
}

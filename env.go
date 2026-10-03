/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file env.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package sicky

import (
	"os"
	"strings"

	"github.com/spf13/viper"

	"github.com/go-sicky/sicky/logger"
)

// sensitiveEnvKeys are configuration keys whose environment override has
// to work even when the key is absent from the config file.
//
// viper's AutomaticEnv only resolves keys that already exist in the
// configuration (Unmarshal walks AllKeys, which merges override, flags,
// bound env vars, the config file and defaults - not the automatic env
// namespace), so without an explicit binding a variable like
// SICKY_MANAGER_AUTH_TOKEN is read by Get() but dropped during
// Unmarshal: the operator gets a silently weaker - or differently
// configured - process than intended.
var sensitiveEnvKeys = []string{
	// Manager: an ignored variable disables the auth gate on /config and
	// /services, or the TLS in front of them.
	"manager.auth_token",
	"manager.expose_config",
	"manager.tls_cert_pem",
	"manager.tls_key_pem",

	// Tracer: DSN and headers carry upstream credentials; sampling
	// controls the export budget.
	"tracer.type",
	"tracer.dsn",
	"tracer.endpoint",
	"tracer.sample_rate",
	"tracer.trust_remote_sampled",
	"tracer.headers",

	// Discovery and messaging selection. registry.redis.password is a
	// credential in the same class as infra.redis.password, so it belongs
	// here too. "registry.type" and "broker.type" were removed: neither
	// registry.Config nor broker.Config declares a Type field, so binding
	// them was a no-op that also made warnIgnoredEnv treat SICKY_REGISTRY_TYPE
	// as a known key - accepted, then silently dropped.
	"registry.local.registry_file_path",
	"registry.redis.password",

	// Infra credentials: an ignored variable means the backend is
	// reached with whatever the file says (often nothing).
	"infra.bun.dsn",
	"infra.clickhouse.dsn",
	"infra.mongo.uri",
	"infra.redis.password",
	"infra.redis.enable_tls",
	"infra.redis.tls_skip_verify",
	"infra.nats.token",
	"infra.nats.password",
	"infra.nats.enable_tls",
	"infra.nats.creds_file",
	"infra.nats.nkey_file",
	"infra.nats.root_ca_file",
	"infra.mqtt.password",
	"infra.mqtt.enable_tls",
	"infra.mqtt.ca_file",
	"infra.s3.access_key",
	"infra.s3.secret_key",
	"infra.s3.session_token",
	"infra.s3.endpoint",
	"infra.elastic.password",
	"infra.elastic.api_key",
	"infra.elastic.service_token",
	"infra.elastic.ca_cert_file",

	// Process-wide behavior.
	"log_level",
}

// envName derives the environment variable viper uses for a
// configuration key (prefix + upper-cased key, dots replaced).
func envName(prefix, key string) string {
	return strings.ToUpper(prefix) + "_" + strings.ToUpper(strings.NewReplacer(".", "_").Replace(key))
}

// bindSensitiveEnv registers an explicit environment binding for every
// key in sensitiveEnvKeys, which is what makes Unmarshal see it.
//
// It binds the flat path only, which is all that can be known before a caller
// hands its target to ConfigUnmarshal. ConfigUnmarshal binds the rest.
func bindSensitiveEnv(v *viper.Viper) {
	bindSensitiveEnvAt(v, "", DefaultEnvPrefix)
}

// bindSensitiveEnvAt is bindSensitiveEnv for a target that nests sicky.Config
// under prefix. Each key is bound with its canonical variable name — derived
// from the key relative to Config, not from the full path — so an operator
// types SICKY_MANAGER_AUTH_TOKEN whether the application nests the framework
// config at the top level or under "sicky". See configPrefixes for why the
// flat path alone was not enough.
func bindSensitiveEnvAt(v *viper.Viper, prefix, envPrefix string) {
	for _, key := range sensitiveEnvKeys {
		bindEnvKey(v, prefix, key, envPrefix)
	}
}

// bindSensitiveEnvFor binds every sensitive key at every prefix where the
// caller's target holds a sicky.Config, and returns the variable names it
// bound so the ignored-variable check does not report them.
//
// The names are returned rather than recomputed because they cannot be
// derived from AllKeys: the variable for a nested key is derived from the
// path relative to Config, not from the full path. Deriving it from
// AllKeys alone would call SICKY_MANAGER_AUTH_TOKEN unknown while it is in
// fact bound and read.
func bindSensitiveEnvFor(v *viper.Viper, raw any, envPrefix string) []string {
	prefixes := configPrefixes(raw)
	bound := make([]string, 0, len(prefixes)*len(sensitiveEnvKeys))

	for _, prefix := range prefixes {
		for _, key := range sensitiveEnvKeys {
			bound = append(bound, bindEnvKey(v, prefix, key, envPrefix))
		}
	}

	return bound
}

// bindEnvKey binds one key at one prefix and returns the variable name it
// bound. A binding failure is logged rather than returned: the alternative is
// ConfigUnmarshal refusing to load a configuration because an environment
// binding failed, which turns a diagnostic into an outage.
func bindEnvKey(v *viper.Viper, prefix, key, envPrefix string) string {
	name := envName(envPrefix, key)

	if err := v.BindEnv(joinKey(prefix, key), name); err != nil {
		logger.Logger.Warn("environment binding failed", "key", joinKey(prefix, key), "error", err.Error())
	}

	return name
}

// ignoredEnv returns the SICKY-style variables that resolve to no known
// configuration key. They cannot be read at all, so keeping quiet about
// them would leave the operator believing an override applied.
//
// bound holds the variable names bound explicitly rather than derived from a
// key path; see bindSensitiveEnvFor.
func ignoredEnv(v *viper.Viper, prefix string, bound []string) []string {
	known := make(map[string]struct{}, len(bound))
	for _, key := range v.AllKeys() {
		known[envName(prefix, key)] = struct{}{}
	}

	for _, name := range bound {
		known[name] = struct{}{}
	}

	ignored := make([]string, 0)
	marker := strings.ToUpper(prefix) + "_"
	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, marker) {
			continue
		}

		if _, ok := known[name]; ok {
			continue
		}

		ignored = append(ignored, name)
	}

	return ignored
}

// warnIgnoredEnv logs every environment variable that will not be read.
func warnIgnoredEnv(v *viper.Viper, prefix string, bound []string) {
	for _, name := range ignoredEnv(v, prefix, bound) {
		logger.Logger.Warn(
			"environment variable matches no configuration key and will be ignored",
			"env", name,
			"hint", "the key must exist in the config file for viper to unmarshal it (or be listed in sicky.sensitiveEnvKeys)",
		)
	}
}

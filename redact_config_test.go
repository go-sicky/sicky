/**
 * @file redact_config_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package sicky

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/registry/consul"
	"github.com/go-sicky/sicky/registry/local"
	"github.com/go-sicky/sicky/registry/redis"
)

// canary is what every credential-bearing field is filled with. One distinct
// value everywhere is deliberate: a leak is found by searching for the string,
// so it does not matter which field it came from.
const canary = "SENTINEL-CANARY"

// registryDir is a directory, not a credential, and is deliberately left
// visible. It appears in sensitiveEnvKeys, which is a different list with a
// different job — "this key must be bindable from the environment" — and
// reading it as "this must be hidden" is how a debugging aid gets redacted
// for no benefit.
const registryDir = "/tmp/sicky/registry/"

// populatedConfig returns a Config with a canary in every field that can carry
// a credential, written out by hand.
//
// Hand-writing it is the point, and the gap is deliberate: a field added later
// is not covered until someone adds it here, and the compiler will not say so.
// The alternative — reflecting the tree and treating every string as a canary —
// would have to decide for itself which strings are secrets, which is the
// judgement that must be made on purpose. The current list is enumerated from
// the Config tree: manager TLS and auth, tracer DSN and headers, each infra and
// registry credential.
func populatedConfig() *Config {
	cfg := &Config{
		Manager: &ManagerConfig{
			AuthToken:  canary,
			TLSCertPEM: "-----BEGIN CERTIFICATE-----\n" + canary + "\n-----END CERTIFICATE-----",
			TLSKeyPEM:  "-----BEGIN PRIVATE KEY-----\n" + canary + "\n-----END PRIVATE KEY-----",
		},
		Tracer: &TracerConfig{
			DSN:     canary,
			Headers: map[string]string{"authorization": canary},
		},
		Infra: &InfraConfig{
			Bun:        &infra.BunConfig{DSN: canary},
			Clickhouse: &infra.ClickHouseConfig{DSN: canary},
			Mongo:      &infra.MongoConfig{URI: canary},
			Redis:      &infra.RedisConfig{Password: canary},
			Nats: &infra.NATSConfig{
				Token:      canary,
				Password:   canary,
				CredsFile:  canary,
				NkeyFile:   canary,
				RootCAFile: canary,
			},
			MQTT: &infra.MQTTConfig{
				Password: canary,
				CAFile:   canary,
			},
			S3: &infra.S3Config{
				AccessKey:    canary,
				SecretKey:    canary,
				SessionToken: canary,
			},
			Elastic: &infra.ElasticConfig{
				Password:     canary,
				APIKey:       canary,
				ServiceToken: canary,
				CACertFile:   canary,
			},
		},
	}

	cfg.Registry.Redis = &redis.Config{Password: canary}
	cfg.Registry.Local = &local.Config{RegistryFilePath: registryDir}
	cfg.Registry.Consul = &consul.Config{}

	return cfg
}

// renderSanitized walks the tree exactly as the Manager's /config handler and
// `sicky config show` do, and returns it as text.
func renderSanitized(t *testing.T, cfg *Config) string {
	t.Helper()

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	out, err := json.MarshalIndent(sanitizeValue("", decoded), "", "  ")
	if err != nil {
		t.Fatalf("marshal sanitized config: %v", err)
	}

	return string(out)
}

// The Manager serves /config when expose_config is true, and `sicky config
// show` prints the same tree. The redaction list is maintained by key name, so
// it can only be as current as the last person to add a field; this asserts
// something about the Config's contents instead.
func TestSanitizedConfigRemovesEveryCredential(t *testing.T) {
	rendered := renderSanitized(t, populatedConfig())

	if strings.Contains(rendered, canary) {
		t.Errorf("a credential survived sanitization:\n%s", rendered)
	}
}

// The canary must actually reach the fields above. A Config that dropped them
// would let the previous test pass for the wrong reason.
func TestPopulatedConfigActuallyCarriesTheCanaries(t *testing.T) {
	cfg := populatedConfig()

	fields := map[string]string{
		"manager.auth_token":          cfg.Manager.AuthToken,
		"manager.tls_cert_pem":        cfg.Manager.TLSCertPEM,
		"manager.tls_key_pem":         cfg.Manager.TLSKeyPEM,
		"tracer.dsn":                  cfg.Tracer.DSN,
		"tracer.headers":              cfg.Tracer.Headers["authorization"],
		"infra.bun.dsn":               cfg.Infra.Bun.DSN,
		"infra.clickhouse.dsn":        cfg.Infra.Clickhouse.DSN,
		"infra.mongo.uri":             cfg.Infra.Mongo.URI,
		"infra.redis.password":        cfg.Infra.Redis.Password,
		"infra.nats.token":            cfg.Infra.Nats.Token,
		"infra.nats.password":         cfg.Infra.Nats.Password,
		"infra.nats.creds_file":       cfg.Infra.Nats.CredsFile,
		"infra.nats.nkey_file":        cfg.Infra.Nats.NkeyFile,
		"infra.nats.root_ca_file":     cfg.Infra.Nats.RootCAFile,
		"infra.mqtt.password":         cfg.Infra.MQTT.Password,
		"infra.mqtt.ca_file":          cfg.Infra.MQTT.CAFile,
		"infra.s3.access_key":         cfg.Infra.S3.AccessKey,
		"infra.s3.secret_key":         cfg.Infra.S3.SecretKey,
		"infra.s3.session_token":      cfg.Infra.S3.SessionToken,
		"infra.elastic.password":      cfg.Infra.Elastic.Password,
		"infra.elastic.api_key":       cfg.Infra.Elastic.APIKey,
		"infra.elastic.service_token": cfg.Infra.Elastic.ServiceToken,
		"infra.elastic.ca_cert_file":  cfg.Infra.Elastic.CACertFile,
		"registry.redis.password":     cfg.Registry.Redis.Password,
	}

	for name, value := range fields {
		if !strings.Contains(value, canary) {
			t.Errorf("%s does not carry the canary, so redaction is not being "+
				"tested for it", name)
		}
	}

	if len(fields) == 0 {
		t.Fatal("no credential fields enumerated: the test would assert nothing")
	}

	// Six of the 33 fragments in utils.secretKeyFragments are redundant —
	// covered by a longer entry's substring match ("token" matches
	// service_token and session_token, "secret" matches secret_key, and so
	// on). Removing those entries does not fail the test above, which is
	// correct rather than a coverage gap: a redundant fragment costs nothing
	// and survives a future rename of the longer one.
	t.Logf("%d credential fields checked", len(fields))
}

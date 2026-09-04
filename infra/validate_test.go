/**
 * @file validate_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package infra

import (
	"errors"
	"strings"
	"testing"
)

// nil configs must stay disabled: Init(nil) returns (nil, nil) and
// Validate on a nil receiver returns nil.
func TestValidateNilConfigs(t *testing.T) {
	if err := (*BadgerConfig)(nil).Validate(); err != nil {
		t.Errorf("Badger nil Validate = %v, want nil", err)
	}
	if err := (*BunConfig)(nil).Validate(); err != nil {
		t.Errorf("Bun nil Validate = %v, want nil", err)
	}
	if err := (*ClickhouseConfig)(nil).Validate(); err != nil {
		t.Errorf("Clickhouse nil Validate = %v, want nil", err)
	}
	if err := (*ElasticConfig)(nil).Validate(); err != nil {
		t.Errorf("Elastic nil Validate = %v, want nil", err)
	}
	if err := (*MongoConfig)(nil).Validate(); err != nil {
		t.Errorf("Mongo nil Validate = %v, want nil", err)
	}
	if err := (*MQTTConfig)(nil).Validate(); err != nil {
		t.Errorf("MQTT nil Validate = %v, want nil", err)
	}
	if err := (*NatsConfig)(nil).Validate(); err != nil {
		t.Errorf("Nats nil Validate = %v, want nil", err)
	}
	if err := (*RedisConfig)(nil).Validate(); err != nil {
		t.Errorf("Redis nil Validate = %v, want nil", err)
	}
	if err := (*RistrettoConfig)(nil).Validate(); err != nil {
		t.Errorf("Ristretto nil Validate = %v, want nil", err)
	}
	if err := (*S3Config)(nil).Validate(); err != nil {
		t.Errorf("S3 nil Validate = %v, want nil", err)
	}
}

// Presence-without-config must abort: every non-nil empty config except
// Ristretto fails Validate.
func TestValidateEmptyConfigsAbort(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
		want error
	}{
		{"badger", func() error { return (&BadgerConfig{}).Ensure().Validate() }, ErrBadgerPathEmpty},
		{"bun", func() error { return (&BunConfig{}).Ensure().Validate() }, ErrBunDSNEmpty},
		{"clickhouse", func() error { return (&ClickhouseConfig{}).Ensure().Validate() }, ErrClickhouseDSNEmpty},
		{"elastic", func() error { return (&ElasticConfig{}).Ensure().Validate() }, ErrElasticNoEndpoint},
		{"elastic blank addr", func() error {
			return (&ElasticConfig{Addresses: []string{"  "}}).Ensure().Validate()
		}, ErrElasticNoEndpoint},
		{"mongo", func() error { return (&MongoConfig{}).Ensure().Validate() }, ErrMongoURIEmpty},
		{"mqtt", func() error { return (&MQTTConfig{}).Ensure().Validate() }, ErrMQTTBrokerEmpty},
		{"nats", func() error { return (&NatsConfig{}).Ensure().Validate() }, ErrNatsURLEmpty},
		{"redis", func() error { return (&RedisConfig{}).Ensure().Validate() }, ErrRedisAddrEmpty},
		{"redis negative db", func() error {
			return (&RedisConfig{Addr: "localhost:6379", DB: -1}).Ensure().Validate()
		}, ErrRedisDBNegative},
		{"redis negative timeout", func() error {
			return (&RedisConfig{Addr: "localhost:6379", DialTimeoutSec: -1}).Ensure().Validate()
		}, ErrRedisTimeoutInvalid},
		{"bun negative pool", func() error {
			return (&BunConfig{Driver: "postgres", DSN: "x", MaxOpenConns: -1}).Ensure().Validate()
		}, ErrBunPoolInvalid},
		{"elastic negative timeout", func() error {
			return (&ElasticConfig{Addresses: []string{"http://localhost:9200"}, TimeoutSec: -1}).Ensure().Validate()
		}, ErrElasticTimeoutInvalid},
		{"s3 negative timeout", func() error {
			return (&S3Config{Region: "us-east-1", Timeout: -1}).Ensure().Validate()
		}, ErrS3TimeoutInvalid},
		{"mqtt negative keepalive", func() error {
			return (&MQTTConfig{Broker: "tcp://localhost:1883", KeepAliveSec: -1}).Ensure().Validate()
		}, ErrMQTTOptionInvalid},
		{"mqtt missing ca file", func() error {
			return (&MQTTConfig{Broker: "tcp://localhost:1883", CAFile: "/nonexistent/ca.pem"}).Ensure().Validate()
		}, ErrMQTTCAUnreadable},
		{"nats negative option", func() error {
			return (&NatsConfig{URL: "nats://localhost:4222", MaxReconnects: -2}).Ensure().Validate()
		}, ErrNatsOptionInvalid}, {"nats missing creds file", func() error {
			return (&NatsConfig{URL: "nats://localhost:4222", CredsFile: "/nonexistent/user.creds"}).Ensure().Validate()
		}, ErrNatsFileUnreadable},
		{"s3", func() error { return (&S3Config{}).Ensure().Validate() }, ErrS3RegionEmpty},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.fn(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// Well-formed configs must pass, including the ristretto all-zero case
// (Ensure fills defaults) and region-only S3 (bucket is optional).
func TestValidateGoodConfigs(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"badger", func() error { return (&BadgerConfig{Path: "/tmp/sicky-badger-test"}).Ensure().Validate() }},
		{"bun pg", func() error {
			return (&BunConfig{Driver: "postgres", DSN: "postgres://u:p@localhost:5432/db?sslmode=disable"}).Ensure().Validate()
		}},
		{"bun empty driver aborts (BREAKING: must be explicit)", func() error {
			if err := (&BunConfig{DSN: "postgres://localhost/db"}).Ensure().Validate(); err == nil {
				return errors.New("want error for empty driver")
			}
			return nil
		}},
		{"bun sqlite memory", func() error {
			return (&BunConfig{Driver: "sqlite", DSN: ":memory:"}).Ensure().Validate()
		}},
		{"clickhouse", func() error {
			return (&ClickhouseConfig{DSN: "clickhouse://localhost:9000/default"}).Ensure().Validate()
		}},
		{"elastic", func() error {
			return (&ElasticConfig{Addresses: []string{"http://localhost:9200"}}).Ensure().Validate()
		}},
		{"elastic cloud only", func() error {
			return (&ElasticConfig{CloudID: "my-deployment:abc123"}).Ensure().Validate()
		}},
		{"mongo", func() error {
			return (&MongoConfig{URI: "mongodb://localhost:27017", DB: "app"}).Ensure().Validate()
		}},
		{"mqtt", func() error {
			return (&MQTTConfig{Broker: "tcp://localhost:1883"}).Ensure().Validate()
		}},
		{"nats", func() error {
			return (&NatsConfig{URL: "nats://localhost:4222"}).Ensure().Validate()
		}},
		{"nats infinite reconnects", func() error {
			return (&NatsConfig{URL: "nats://localhost:4222", MaxReconnects: -1}).Ensure().Validate()
		}},
		{"redis", func() error {
			return (&RedisConfig{Addr: "localhost:6379"}).Ensure().Validate()
		}},
		{"ristretto all zero", func() error { return (&RistrettoConfig{}).Ensure().Validate() }},
		{"s3 region only, no bucket", func() error {
			return (&S3Config{Region: "us-east-1"}).Ensure().Validate()
		}},
		{"s3 minio", func() error {
			return (&S3Config{Region: "us-east-1", Endpoint: "http://localhost:9000", UsePathStyle: true}).Ensure().Validate()
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.fn(); err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateBunUnknownDriver(t *testing.T) {
	err := (&BunConfig{Driver: "postgrse", DSN: "x"}).Ensure().Validate()
	if !errors.Is(err, ErrBunUnsupportedDriver) {
		t.Errorf("Validate() = %v, want %v", err, ErrBunUnsupportedDriver)
	}
}

// Init(nil) must stay a no-op so unset infra sections are skipped.
func TestInitNilIsNoop(t *testing.T) {
	if _, err := InitBadger(nil); err != nil {
		t.Errorf("InitBadger(nil) = %v, want nil", err)
	}
	if _, err := InitBun(nil); err != nil {
		t.Errorf("InitBun(nil) = %v, want nil", err)
	}
	if _, err := InitClickhouse(nil); err != nil {
		t.Errorf("InitClickhouse(nil) = %v, want nil", err)
	}
	if _, err := InitElastic(nil); err != nil {
		t.Errorf("InitElastic(nil) = %v, want nil", err)
	}
	if _, err := InitMongo(nil); err != nil {
		t.Errorf("InitMongo(nil) = %v, want nil", err)
	}
	if _, err := InitMQTT(nil); err != nil {
		t.Errorf("InitMQTT(nil) = %v, want nil", err)
	}
	if _, err := InitNats(nil); err != nil {
		t.Errorf("InitNats(nil) = %v, want nil", err)
	}
	if _, err := InitRedis(nil); err != nil {
		t.Errorf("InitRedis(nil) = %v, want nil", err)
	}
	if _, err := InitRistretto(nil); err != nil {
		t.Errorf("InitRistretto(nil) = %v, want nil", err)
	}
	if _, err := InitS3(nil); err != nil {
		t.Errorf("InitS3(nil) = %v, want nil", err)
	}
}

// Empty configs must fail fast without dialing anything.
func TestInitEmptyAbortsFast(t *testing.T) {
	if _, err := InitBadger(&BadgerConfig{}); !errors.Is(err, ErrBadgerPathEmpty) {
		t.Errorf("InitBadger({}) = %v, want %v", err, ErrBadgerPathEmpty)
	}
	if _, err := InitBun(&BunConfig{}); !errors.Is(err, ErrBunDSNEmpty) {
		t.Errorf("InitBun({}) = %v, want %v", err, ErrBunDSNEmpty)
	}
	if _, err := InitClickhouse(&ClickhouseConfig{}); !errors.Is(err, ErrClickhouseDSNEmpty) {
		t.Errorf("InitClickhouse({}) = %v, want %v", err, ErrClickhouseDSNEmpty)
	}
	if _, err := InitElastic(&ElasticConfig{}); !errors.Is(err, ErrElasticNoEndpoint) {
		t.Errorf("InitElastic({}) = %v, want %v", err, ErrElasticNoEndpoint)
	}
	if _, err := InitMongo(&MongoConfig{}); !errors.Is(err, ErrMongoURIEmpty) {
		t.Errorf("InitMongo({}) = %v, want %v", err, ErrMongoURIEmpty)
	}
	if _, err := InitMQTT(&MQTTConfig{}); !errors.Is(err, ErrMQTTBrokerEmpty) {
		t.Errorf("InitMQTT({}) = %v, want %v", err, ErrMQTTBrokerEmpty)
	}
	if _, err := InitNats(&NatsConfig{}); !errors.Is(err, ErrNatsURLEmpty) {
		t.Errorf("InitNats({}) = %v, want %v", err, ErrNatsURLEmpty)
	}
	if _, err := InitRedis(&RedisConfig{}); !errors.Is(err, ErrRedisAddrEmpty) {
		t.Errorf("InitRedis({}) = %v, want %v", err, ErrRedisAddrEmpty)
	}
	if _, err := InitS3(&S3Config{}); !errors.Is(err, ErrS3RegionEmpty) {
		t.Errorf("InitS3({}) = %v, want %v", err, ErrS3RegionEmpty)
	}
}

// Ensure fills timing defaults from zero but preserves explicit negatives
// so Validate aborts instead of silently swallowing them.
func TestEnsureTimingDefaults(t *testing.T) {
	r := (&RedisConfig{Addr: "localhost:6379"}).Ensure()
	if r.DialTimeoutSec != DefaultRedisDialTimeoutSec ||
		r.ReadTimeoutSec != DefaultRedisReadTimeoutSec ||
		r.WriteTimeoutSec != DefaultRedisWriteTimeoutSec {
		t.Errorf("Redis Ensure defaults = %d/%d/%d, want %d/%d/%d",
			r.DialTimeoutSec, r.ReadTimeoutSec, r.WriteTimeoutSec,
			DefaultRedisDialTimeoutSec, DefaultRedisReadTimeoutSec, DefaultRedisWriteTimeoutSec)
	}
	if err := r.Validate(); err != nil {
		t.Errorf("Redis Ensure+Validate = %v, want nil", err)
	}

	m := (&MQTTConfig{Broker: "tcp://localhost:1883"}).Ensure()
	if m.KeepAliveSec != DefaultMQTTKeepAliveSec || m.ConnectTimeoutSec != DefaultMQTTConnectTimeoutSec {
		t.Errorf("MQTT Ensure defaults = %d/%d, want %d/%d",
			m.KeepAliveSec, m.ConnectTimeoutSec,
			DefaultMQTTKeepAliveSec, DefaultMQTTConnectTimeoutSec)
	}

	n := (&NatsConfig{URL: "nats://localhost:4222"}).Ensure()
	if n.TimeoutSec != DefaultNatsTimeoutSec ||
		n.ReconnectWaitSec != DefaultNatsReconnectWaitSec ||
		n.MaxReconnects != DefaultNatsMaxReconnects {
		t.Errorf("Nats Ensure defaults = %d/%d/%d, want %d/%d/%d",
			n.TimeoutSec, n.ReconnectWaitSec, n.MaxReconnects,
			DefaultNatsTimeoutSec, DefaultNatsReconnectWaitSec, DefaultNatsMaxReconnects)
	}
	if err := n.Validate(); err != nil {
		t.Errorf("Nats Ensure+Validate = %v, want nil", err)
	}

	e := (&ElasticConfig{Addresses: []string{"http://localhost:9200"}}).Ensure()
	if e.TimeoutSec != DefaultElasticTimeoutSec {
		t.Errorf("Elastic Ensure TimeoutSec = %d, want %d", e.TimeoutSec, DefaultElasticTimeoutSec)
	}

	s := (&S3Config{Region: "us-east-1"}).Ensure()
	if s.Timeout != DefaultInitTimeoutSec {
		t.Errorf("S3 Ensure Timeout = %d, want %d", s.Timeout, DefaultInitTimeoutSec)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("S3 region-only Validate = %v, want nil", err)
	}
}

// GetMongoDB resolves explicit name > cfg.DB > URI path; nil when nothing.
func TestGetMongoDBResolution(t *testing.T) {
	if GetMongoDB("anything") != nil {
		t.Error("GetMongoDB with nil client = non-nil, want nil")
	}

	if got := effectiveMongoDB(&MongoConfig{URI: "mongodb://h:27017", DB: "cfgdb"}); got != "cfgdb" {
		t.Errorf("effectiveMongoDB cfg.DB wins = %q, want cfgdb", got)
	}
	if got := effectiveMongoDB(&MongoConfig{URI: "mongodb://h:27017/uripath"}); got != "uripath" {
		t.Errorf("effectiveMongoDB URI path fallback = %q, want uripath", got)
	}
	if got := effectiveMongoDB(&MongoConfig{URI: "mongodb://h:27017"}); got != "" {
		t.Errorf("effectiveMongoDB no db = %q, want empty", got)
	}
	if got := effectiveMongoDB(nil); got != "" {
		t.Errorf("effectiveMongoDB nil = %q, want empty", got)
	}
}

// Ristretto all-zero must work via Ensure defaults, without external I/O.
func TestInitRistrettoZeroConfig(t *testing.T) {
	c, err := InitRistretto(&RistrettoConfig{})
	if err != nil {
		t.Fatalf("InitRistretto({}) = %v, want nil", err)
	}
	if c == nil {
		t.Fatal("InitRistretto({}) = nil cache, want non-nil")
	}
	if GetRistretto() == nil {
		t.Error("GetRistretto() = nil after Init, want non-nil")
	}
	c.Close()
	ClearRistretto()
	if GetRistretto() != nil {
		t.Error("GetRistretto() != nil after Clear, want nil")
	}
}

// Credentials must never survive into log fields.
func TestRedactDSN(t *testing.T) {
	cases := []struct {
		in       string
		contains string // must still be visible
	}{
		{"clickhouse://user:s3cret@localhost:9000/default", "localhost:9000"},
		{"mongodb://admin:p%40ss@mongo:27017/app", "mongo:27017"},
		{"postgres://u:p@localhost:5432/db?sslmode=disable", "localhost:5432"},
	}

	for _, c := range cases {
		got := redactDSN(c.in)
		if !strings.Contains(got, c.contains) {
			t.Errorf("redactDSN(%q) = %q, want it to contain %q", c.in, got, c.contains)
		}
		for _, secret := range []string{"s3cret", "p%40ss", ":p@"} {
			if strings.Contains(got, secret) {
				t.Errorf("redactDSN(%q) = %q, leaks credential %q", c.in, got, secret)
			}
		}
	}

	if got := redactDSN(""); got != "" {
		t.Errorf("redactDSN(\"\") = %q, want empty", got)
	}
	if got := redactDSN("://not a url"); got != "REDACTED" {
		t.Errorf("redactDSN(garbage) = %q, want REDACTED", got)
	}
}

/**
 * @file redis_instances_test.go
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

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"

	"github.com/go-sicky/sicky/metrics"
)

func TestRedisConfigInstanceRequiresCompleteConfig(t *testing.T) {
	cfg := &RedisConfig{
		Addr: "127.0.0.1:6379",
		Instances: map[string]*RedisConfig{
			"cache": {DB: 1},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrRedisAddrEmpty) {
		t.Fatalf("Validate() = %v, want ErrRedisAddrEmpty for the incomplete instance", err)
	}

	if err == nil || !strings.Contains(err.Error(), "instances.cache") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

func TestRedisConfigRejectsNestedInstances(t *testing.T) {
	cfg := &RedisConfig{
		Addr: "127.0.0.1:6379",
		Instances: map[string]*RedisConfig{
			"cache": {
				Addr:      "127.0.0.1:6379",
				NoDefault: true,
			},
		},
	}

	if err := cfg.Validate(); !errors.Is(err, ErrInstanceNested) {
		t.Fatalf("Validate() = %v, want ErrInstanceNested", err)
	}
}

// Shape errors must be caught per instance and name the instance, so an
// operator does not have to guess which block is wrong.
func TestRedisConfigInstanceShapeIsValidated(t *testing.T) {
	cfg := &RedisConfig{
		Addr: "127.0.0.1:6379",
		Instances: map[string]*RedisConfig{
			"cache": {Addr: "127.0.0.1:6379", PoolSize: 2, MinIdleConns: 3},
		},
	}

	err := cfg.Validate()
	if !errors.Is(err, ErrRedisIdleExceedsPool) {
		t.Fatalf("Validate() = %v, want ErrRedisIdleExceedsPool", err)
	}

	if !strings.Contains(err.Error(), "instances.cache") {
		t.Fatalf("Validate() = %v, want the instance name in the error", err)
	}
}

// resolveInstances validates a copy: Ensure fills timing defaults, and the
// caller's map is the caller's property.
func TestRedisConfigInstancesDoNotMutateCaller(t *testing.T) {
	cfg := &RedisConfig{
		Instances: map[string]*RedisConfig{
			"cache": {Addr: "127.0.0.1:6379"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if got := cfg.Instances["cache"].DialTimeoutSec; got != 0 {
		t.Fatalf("Validate() wrote %d into the caller's instance config", got)
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		t.Fatalf("resolveInstances() = %v, want nil", err)
	}

	if len(names) != 1 || names[0] != "cache" {
		t.Fatalf("resolveInstances() names = %v, want [cache]", names)
	}

	if got := instances["cache"].DialTimeoutSec; got != DefaultRedisDialTimeoutSec {
		t.Fatalf("resolved instance DialTimeoutSec = %d, want %d", got, DefaultRedisDialTimeoutSec)
	}
}

func TestRedisValidateDefaultSelection(t *testing.T) {
	instances := map[string]*RedisConfig{
		"cache": {Addr: "127.0.0.1:6379"},
		"queue": {Addr: "127.0.0.1:6380"},
	}

	if err := (&RedisConfig{Instances: instances}).Validate(); err != nil {
		t.Errorf("incomplete top-level + instances = %v, want nil (the first sorted instance is the default)", err)
	}

	if err := (&RedisConfig{DefaultInstance: "queue", Instances: instances}).Validate(); err != nil {
		t.Errorf("explicit default_instance = %v, want nil", err)
	}

	if err := (&RedisConfig{DefaultInstance: "missing", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceNotFound) {
		t.Errorf("unknown default_instance = %v, want ErrDefaultInstanceNotFound", err)
	}

	if err := (&RedisConfig{NoDefault: true, Instances: instances}).Validate(); err != nil {
		t.Errorf("no_default with instances = %v, want nil", err)
	}

	if err := (&RedisConfig{NoDefault: true}).Validate(); !errors.Is(err, ErrInstancesEmpty) {
		t.Errorf("no_default without instances = %v, want ErrInstancesEmpty", err)
	}

	if err := (&RedisConfig{NoDefault: true, DefaultInstance: "cache", Instances: instances}).Validate(); !errors.Is(err, ErrDefaultInstanceConflict) {
		t.Errorf("no_default + default_instance = %v, want ErrDefaultInstanceConflict", err)
	}
}

// InitRedis must fail fast and register nothing: an unreachable server is an
// ordinary misconfiguration, and a half-registered component cannot be
// replaced later because every registration is first-wins.
func TestInitRedisFailFast(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *RedisConfig
		want string
	}{
		{
			name: "top-level",
			cfg: &RedisConfig{
				Addr: "127.0.0.1:1",
				Instances: map[string]*RedisConfig{
					"cache": {Addr: "127.0.0.1:1"},
				},
			},
		},
		{
			name: "named-only",
			cfg: &RedisConfig{
				Instances: map[string]*RedisConfig{
					"cache": {Addr: "127.0.0.1:1"},
				},
			},
			want: "redis instance cache",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ShutdownRedis()
			t.Cleanup(func() { ShutdownRedis() })

			client, err := InitRedis(tc.cfg)
			if err == nil {
				t.Fatalf("InitRedis() = %v, want a connection failure", client)
			}

			if client != nil {
				t.Fatalf("InitRedis() returned %v alongside the error, want nil", client)
			}

			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("InitRedis() = %v, want it to name the failing instance", err)
			}

			if GetRedis() != nil {
				t.Fatal("GetRedis() = non-nil after a failed init")
			}

			if got := GetRedisInstances(); len(got) != 0 {
				t.Fatalf("GetRedisInstances() = %v, want none after a failed init", got)
			}
		})
	}
}

// ShutdownRedis follows the same generic contract as ShutdownBun: a default
// alias is closed through the named loop, never twice. The clients are built
// directly because the contract needs no live server — Close on an unused pool
// is enough to prove which handles the loop reaches.
func TestShutdownRedisClosesDefaultAliasOnce(t *testing.T) {
	ShutdownRedis()
	t.Cleanup(func() { ShutdownRedis() })

	primary := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	secondary := redis.NewClient(&redis.Options{Addr: "127.0.0.1:2"})

	if _, stored := redisInstances.putFirst("primary", primary); !stored {
		t.Fatal("could not register the primary instance")
	}

	redisInstances.putFirst("secondary", secondary)

	mu.Lock()
	Redis = primary
	redisDefaultInstance = "primary"
	mu.Unlock()

	if defaultNeedsClose("primary", GetRedisInstances()) {
		t.Fatal("the default alias must not be closed a second time")
	}

	if err := ShutdownRedis(); err != nil {
		t.Fatalf("ShutdownRedis() = %v, want nil", err)
	}

	if GetRedis() != nil || len(GetRedisInstances()) != 0 {
		t.Fatal("ShutdownRedis must clear the alias and every named instance")
	}
}

// The hook label is what keeps one tenant's latency series separate from
// another's; the zero-value hook must keep reporting the shared series,
// because existing call sites and tests construct it without a label.
func TestRedisMetricsHookUsesInstanceLabel(t *testing.T) {
	if got := (redisMetricsHook{}).component(); got != "redis" {
		t.Fatalf("zero-value hook label = %q, want redis", got)
	}

	if got := (redisMetricsHook{label: "redis:cache"}).component(); got != "redis:cache" {
		t.Fatalf("named hook label = %q, want redis:cache", got)
	}

	const label = "redis:cache"

	before := testutil.ToFloat64(metrics.InfraOpsTotal.WithLabelValues(label, "GET", "ok"))

	hook := redisMetricsHook{label: label}
	cmd := redis.NewCmd(context.Background(), "get", "key")
	if err := hook.ProcessHook(func(context.Context, redis.Cmder) error { return nil })(context.Background(), cmd); err != nil {
		t.Fatalf("hook returned %v", err)
	}

	after := testutil.ToFloat64(metrics.InfraOpsTotal.WithLabelValues(label, "GET", "ok"))
	if after != before+1 {
		t.Fatalf("sicky_infra_ops_total{infra=%q,op=\"GET\"} = %v, want %v", label, after, before+1)
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

/**
 * @file redis_metrics_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package infra

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"

	"github.com/go-sicky/sicky/metrics"
)

// fireProcessHook drives the hook the way go-redis does, with name as
// the first argument — which is exactly how go-redis derives
// cmd.Name(). A typed method passes a compile-time constant here; a Do
// or NewCmd caller does not.
func fireProcessHook(t *testing.T, name string) {
	t.Helper()

	cmd := redis.NewCmd(t.Context(), name, "arg")

	hook := redisMetricsHook{}
	if err := hook.ProcessHook(func(context.Context, redis.Cmder) error { return nil })(t.Context(), cmd); err != nil {
		t.Fatalf("hook returned %v", err)
	}
}

func redisOpCount(t *testing.T, op string) float64 {
	t.Helper()

	return testutil.ToFloat64(metrics.InfraOpsTotal.WithLabelValues("redis", op, "ok"))
}

// TestRedisHookBoundsCallerSuppliedCommandNames is the B11 regression.
// cmd.Name() is the first argument, so before the fix each distinct
// caller-supplied name minted a permanent Prometheus series: an
// unbounded registry that eventually OOMs the process or times out
// every scrape. The fix must route them all onto one shared series.
func TestRedisHookBoundsCallerSuppliedCommandNames(t *testing.T) {
	names := []string{"tenant:42:shard-1", "shard-7", "CUSTOM.LOAD", "another:one"}

	before := redisOpCount(t, metrics.UnknownCommand)

	for _, name := range names {
		fireProcessHook(t, name)
	}

	got := redisOpCount(t, metrics.UnknownCommand) - before
	if got != float64(len(names)) {
		t.Fatalf("sicky_infra_ops_total{op=%q} grew by %v after %d distinct caller-supplied names, want %v: "+
			"they must all collapse onto one shared series", metrics.UnknownCommand, got, len(names), float64(len(names)))
	}

	// The decisive check: none of the raw names may exist as its own
	// label. Reading a child is enough — GetMetricWithLabelValues
	// creates it on first use, so read before and after instead.
	for _, name := range names {
		if got := redisOpCount(t, name); got != 0 {
			t.Fatalf("a series was minted for the caller-supplied name %q (count %v): the label is unbounded", name, got)
		}
	}
}

// TestRedisHookKeepsKnownVerbLabels pins the compatibility half: every
// typed go-redis method must keep its byte-identical label, so a
// dashboard or alert built on sicky_infra_ops_total{op="HGETALL"} keeps
// working.
func TestRedisHookKeepsKnownVerbLabels(t *testing.T) {
	before := redisOpCount(t, "HGETALL")
	fireProcessHook(t, "HGETALL")

	if got := redisOpCount(t, "HGETALL"); got != before+1 {
		t.Fatalf("sicky_infra_ops_total{op=\"HGETALL\"} = %v, want %v: a known verb must keep its own label",
			got, before+1)
	}
}

// TestRedisHookUppercasesKnownVerbs pins the case handling. go-redis
// lower-cases the name it derives, so a typed method's label arrives
// already upper-cased; normalization must not drop it to OTHER.
func TestRedisHookUppercasesKnownVerbs(t *testing.T) {
	before := redisOpCount(t, "GET")
	fireProcessHook(t, "get")

	if got := redisOpCount(t, "GET"); got != before+1 {
		t.Fatalf("sicky_infra_ops_total{op=\"GET\"} = %v, want %v: a lower-case known verb must normalize to GET",
			got, before+1)
	}
}

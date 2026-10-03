/**
 * @file pair_cache_test.go
 * @package metrics
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package metrics

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestPairCache builds a cache over fresh Vecs so a test can observe
// the children directly without touching the package-level metrics,
// which are shared with every other test in the binary.
func newTestPairCache(totalLabels, durLabels int) *pairCache {
	totalNames := make([]string, totalLabels)
	for i := range totalNames {
		totalNames[i] = "l"
	}

	durNames := make([]string, durLabels)
	for i := range durNames {
		durNames[i] = "l"
	}

	return newPairCache(
		prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_total"}, totalNames),
		prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_seconds"}, durNames),
		totalLabels, durLabels,
	)
}

func (c *pairCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.m)
}

// newCappedPairCache is newTestPairCache with a deliberately tiny cap, so the
// saturation behavior can be exercised without allocating pairCacheMax
// entries.
func newCappedPairCache(totalLabels, durLabels, limit int) *pairCache {
	c := newTestPairCache(totalLabels, durLabels)
	c.max = limit

	return c
}

func TestPairCacheReusesChildrenForSameLabels(t *testing.T) {
	c := newTestPairCache(3, 2)

	first, firstObs := c.resolve("redis", "GET", "", "")
	for range 5 {
		got, gotObs := c.resolve("redis", "GET", "", "")
		if got != first || gotObs != firstObs {
			t.Fatal("the same label set must resolve to the identical counter and observer, not a fresh child")
		}
	}

	if got := c.size(); got != 1 {
		t.Fatalf("cached children = %d, want 1: repeating a label set must not grow the cache", got)
	}
}

func TestPairCacheSeparatesDistinctLabels(t *testing.T) {
	c := newTestPairCache(3, 2)

	// result is the THIRD label on the counter (infra, op, result), so
	// that is the slot that has to differ; the fourth is only ever
	// consumed by the widest counter (server, method, route, code).
	ok, okObs := c.resolve("redis", "GET", "ok", "")
	fail, failObs := c.resolve("redis", "GET", "err", "")

	if ok == fail {
		t.Fatal("label sets that differ only in the trailing result must not share a counter, or error counts would land in the success series")
	}

	// The histogram is not labeled by result, so both calls legitimately
	// resolve the same observer. Pinning that here documents the
	// relationship between the two arities instead of leaving it implied.
	if okObs != failObs {
		t.Fatal("the histogram takes no result label, so both label sets must resolve the same observer")
	}

	if got := c.size(); got != 2 {
		t.Fatalf("cached children = %d, want 2", got)
	}
}

func TestPairCacheDelegatesArityValidationToPrometheus(t *testing.T) {
	// Deliberately mismatched arities: a 3-label counter paired with a
	// durN of 2. resolve must hand WithLabelValues exactly durN values,
	// and prometheus must reject the count rather than the series being
	// silently mislabelled.
	total := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "arity_total"}, []string{"a", "b", "c"})
	dur := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "arity_seconds"}, []string{"a", "b"})
	c := newPairCache(total, dur, 3, 3)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("an arity mismatch must panic: silently mislabelling a series is worse than a loud failure")
		}
	}()

	c.resolve("a", "b", "c", "d")
}

func TestPairCacheSupportsSingleLabelHistogram(t *testing.T) {
	// The job pair is the one shape whose two arities differ by more
	// than one: JobRunsTotal takes (job, task, result) while
	// JobRunDuration takes only (job). Pin it so a future refactor that
	// derives one arity from the other cannot break it -- that mistake
	// panics on the very first job run.
	c := newTestPairCache(3, 1)

	counter, observer := c.resolve("nightly", "export", "ok", "")
	if counter == nil || observer == nil {
		t.Fatal("resolve returned a nil child for a never-seen label set")
	}

	counter.Inc()
	observer.Observe(0.5)

	if got := c.size(); got != 1 {
		t.Fatalf("cached children = %d, want 1", got)
	}
}

func TestPairCacheConcurrentResolveIsRaceFree(t *testing.T) {
	c := newTestPairCache(3, 2)

	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			// Every worker uses a distinct label set, so all of them take
			// the miss path and race for the write lock; the rest repeat
			// one shared set to hammer the read lock.
			counter, observer := c.resolve("redis", string(rune('a'+i%8)), "", "")
			counter.Inc()
			observer.Observe(0.01)
		})
	}
	wg.Wait()

	if got := c.size(); got != 8 {
		t.Fatalf("cached children = %d, want 8: one per distinct label set", got)
	}
}

// A saturated cache must drop observations instead of growing the registry:
// the CounterVec/HistogramVec children a pair holds are permanent once created,
// so an unbounded table is an unbounded registry. It must also report that it
// dropped them rather than losing them silently.
func TestPairCacheStopsGrowingAtCap(t *testing.T) {
	c := newCappedPairCache(3, 2, 8)

	for i := range 500 {
		counter, observer := c.resolve("redis", strconv.Itoa(i), "", "")
		counter.Inc()
		observer.Observe(0.01)
	}

	if got := c.size(); got > 8 {
		t.Fatalf("cached children = %d, want at most the cap of 8", got)
	}

	before := testutil.ToFloat64(labelSetsDroppedTotal)
	if before == 0 {
		t.Fatal("a saturated cache must count the dropped observations")
	}
}

// A label set that was already admitted must keep resolving to its own child
// once the table is full — saturation must not start handing an existing
// series a different metric object.
func TestPairCacheServesAdmittedLabelsAfterCap(t *testing.T) {
	c := newCappedPairCache(3, 2, 4)

	counter, observer := c.resolve("redis", "GET", "", "")

	for i := range 100 {
		dropped, droppedObs := c.resolve("redis", strconv.Itoa(i), "", "")
		dropped.Inc()
		droppedObs.Observe(0.01)
	}

	got, gotObs := c.resolve("redis", "GET", "", "")
	if got != counter || gotObs != observer {
		t.Fatal("an admitted label set must still resolve to its original child after the cache saturates")
	}
}

// The cache key is a digest of the label tuple, so field boundaries have to be
// unambiguous: without a separator these two tuples collide and their
// observations land in one series.
func TestPairCacheKeySeparatesFields(t *testing.T) {
	c := newTestPairCache(3, 2)

	first, _ := c.resolve("a", "bc", "", "")
	second, _ := c.resolve("ab", "c", "", "")

	if first == second {
		t.Fatal(`("a","bc") and ("ab","c") must not share a counter`)
	}

	if got := c.size(); got != 2 {
		t.Fatalf("cached children = %d, want 2", got)
	}
}

// resolveUncached is what the Observe* helpers paid before pairCache
// existed: two WithLabelValues calls per operation. prometheus memoizes
// children internally, so this returns the same metric objects as
// resolve -- which is exactly why the cache can only be judged on time,
// see the benchmarks below, and not on identity.
func resolveUncached(c *pairCache, a, b, cc, d string) (prometheus.Counter, prometheus.Observer) {
	labels := [4]string{a, b, cc, d}

	return c.total.WithLabelValues(labels[:c.totalN]...),
		c.dur.WithLabelValues(labels[:c.durN]...)
}

func BenchmarkPairCacheHit(b *testing.B) {
	c := newTestPairCache(3, 2)
	c.resolve("redis", "GET", "ok", "")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		counter, observer := c.resolve("redis", "GET", "ok", "")
		counter.Inc()
		observer.Observe(0.001)
	}
}

func BenchmarkPairCacheMissPath(b *testing.B) {
	c := newTestPairCache(3, 2)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		counter, observer := resolveUncached(c, "redis", "GET", "ok", "")
		counter.Inc()
		observer.Observe(0.001)
	}
}

func TestPairCacheHelpersPopulateTheCaches(t *testing.T) {
	// The public helpers must go through the caches rather than calling
	// WithLabelValues themselves, otherwise the whole optimization is
	// bypassed. This is a structural check: after a round of real
	// observations the corresponding cache must be populated.
	ObserveInfraOp("redis", "get", time.Time{}, nil)
	ObserveServerRequest("http", "GET", "/x", "200", 0)
	ObserveBrokerPublish("nsq", "orders", time.Time{}, nil)
	ObserveJobRun("nightly", "export", "ok", 0)

	for name, c := range map[string]*pairCache{
		"infraOpCache":    infraOpCache,
		"serverReqCache":  serverReqCache,
		"brokerPubCache":  brokerPubCache,
		"jobRunCache":     jobRunCache,
		"clientReqCache":  clientReqCache,
		"brokerHndCache":  brokerHndCache,
		"registryOpCache": registryOpCache,
	} {
		if c.size() == 0 {
			t.Fatalf("%s is empty after an observation: the helper bypassed the pair cache", name)
		}
	}
}

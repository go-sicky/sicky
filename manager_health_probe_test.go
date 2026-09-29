/**
 * Copyright 2026 The sicky Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @file manager_health_probe_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package sicky

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHealthConcurrentProbesCollapse pins the A4 guarantee: /health and
// /ready are unauthenticated and each pass fans out to 10 goroutines
// issuing up to 6 backend pings, so concurrent requests must share one
// pass instead of multiplying that fan-out.
//
// The observation is the business checker's own invocation count — a
// user-visible effect, deliberately not the singleflight group itself,
// which would only prove the mechanism rather than the outcome.
func TestHealthConcurrentProbesCollapse(t *testing.T) {
	m := NewManager(nil, "app", "v1")

	var calls atomic.Int64
	// Block the first caller inside the pass so the concurrent requests
	// are guaranteed to land inside the same in-flight window rather than
	// racing past it and each triggering a fresh pass of their own.
	release := make(chan struct{})
	entered := make(chan struct{})

	RegisterHealthChecker("biz-collapse", func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}

		return nil
	})

	defer UnregisterHealthChecker("biz-collapse")

	const requests = 24

	var wg sync.WaitGroup

	wg.Add(requests)

	for range requests {
		go func() {
			defer wg.Done()

			rec := httptest.NewRecorder()
			m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))
		}()
	}

	// Wait until the leader is parked inside the checker, then give the
	// remaining goroutines time to reach the handler.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the first health pass never reached the business checker")
	}

	time.Sleep(150 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("concurrent health probes ran %d passes, want 1: "+
			"unauthenticated concurrent requests must not multiply the backend fan-out", got)
	}
}

// TestHealthSequentialProbesAreFresh is the other half of the A4
// contract: deduplication must never become caching. A request that
// arrives after the previous pass finished has to get its own real-time
// probe, so a dependency that goes down is still reported on the very
// next scrape rather than after some TTL.
func TestHealthSequentialProbesAreFresh(t *testing.T) {
	m := NewManager(nil, "app", "v1")

	var calls atomic.Int64
	RegisterHealthChecker("biz-fresh", func(ctx context.Context) error {
		calls.Add(1)

		return nil
	})

	defer UnregisterHealthChecker("biz-fresh")

	for i := range 3 {
		rec := httptest.NewRecorder()
		m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

		if rec.Code != http.StatusOK {
			t.Fatalf("pass %d: status %d, want %d", i, rec.Code, http.StatusOK)
		}

		if got := calls.Load(); got != int64(i+1) {
			t.Fatalf("after %d sequential probes the checker ran %d times: "+
				"a sequential caller must get a fresh pass, not a cached result", i+1, got)
		}
	}
}

// TestHealthProbeReflectsStateChange pins the practical consequence of
// TestHealthSequentialProbesAreFresh: a dependency failing between two
// probes must be visible on the immediately following request, with no
// warm-up delay and no TTL to wait out.
func TestHealthProbeReflectsStateChange(t *testing.T) {
	m := NewManager(nil, "app", "v1")

	var fail atomic.Bool
	RegisterHealthChecker("biz-toggling", func(ctx context.Context) error {
		if fail.Load() {
			return errTestBoom
		}

		return nil
	})

	defer UnregisterHealthChecker("biz-toggling")

	rec := httptest.NewRecorder()
	m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("healthy checker must report 200, got %d", rec.Code)
	}

	fail.Store(true)

	rec = httptest.NewRecorder()
	m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a checker that just started failing must degrade the very next probe, got %d", rec.Code)
	}
}

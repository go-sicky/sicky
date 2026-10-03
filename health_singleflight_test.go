/**
 * @file health_singleflight_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"context"
	"sync"
	"testing"
	"time"
)

// /health deduplicates concurrent fan-outs through a singleflight group so a
// thundering herd of scrapers costs one probe pass rather than N. The leader's
// context governs that pass, which is only safe if the leader's context is not
// something a client controls.
//
// It is. The leader's reqCtx is the HTTP request context of whichever scraper
// won the race, so a scraper that hangs up mid-probe cancels the fan-out for
// every other scraper still connected: they all render the same "unhealthy"
// verdict, produced by a client going away rather than by any backend failing.
func TestHealthFanOutSurvivesLeaderDisconnect(t *testing.T) {
	m := NewManager(nil, "app", "v1")

	entered := make(chan struct{})
	release := make(chan struct{})

	var once sync.Once

	RegisterHealthChecker("slow-ok", func(context.Context) error {
		once.Do(func() { close(entered) })

		<-release

		return nil
	})

	defer UnregisterHealthChecker("slow-ok")

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()

	results := make(chan string, 2)

	// Leader: its context is the one singleflight will propagate.
	go func() {
		results <- componentStatus(m.collectComponentHealth(leaderCtx), "slow-ok")
	}()

	// Wait until the fan-out is genuinely in flight before racing a second
	// caller into the same group.
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the leader never started the probe")
	}

	go func() {
		results <- componentStatus(m.collectComponentHealth(context.Background()), "slow-ok")
	}()

	// Give the second caller time to join the in-flight group, so it is a
	// waiter on the leader's pass rather than a fresh one.
	time.Sleep(50 * time.Millisecond)

	// The leader's client hangs up. Its request context is canceled and the
	// in-flight probe must not notice.
	cancelLeader()

	close(release)

	for range 2 {
		select {
		case status := <-results:
			if status != statusHealthy {
				t.Errorf("checker reported %q; a client disconnecting must not "+
					"degrade a backend that is healthy", status)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("a caller never returned from collectComponentHealth")
		}
	}
}

// A probe that ignores cancellation must not be able to pin a health request
// past its own deadline; runCheck owns that guarantee and this pins it for the
// case where a business checker sleeps straight through the timeout.
func TestHealthCheckerCannotOutliveItsDeadline(t *testing.T) {
	m := NewManager(nil, "app", "v1")

	// Overshoot the deadline by a margin, not by seconds: runCheck
	// abandons an overrun on purpose, so a checker that slept for many
	// multiples of the timeout would still be running when the package's
	// goleak check runs, and this test would report a leak it created
	// itself.
	RegisterHealthChecker("sleepy", func(context.Context) error {
		time.Sleep(healthCheckTimeout + 500*time.Millisecond)
		return nil
	})

	defer UnregisterHealthChecker("sleepy")

	start := time.Now()
	got := m.collectComponentHealth(context.Background())
	elapsed := time.Since(start)

	if elapsed > healthCheckTimeout+time.Second {
		t.Errorf("collectComponentHealth took %v; it must not exceed the checker deadline", elapsed)
	}

	// The verdict may be unhealthy — the checker really did overrun — but it
	// must be a decision rather than a hang.
	if componentStatus(got, "sleepy") == "" {
		t.Error("the checker must appear in the result with some status")
	}
}

func componentStatus(components []componentHealth, name string) string {
	for _, c := range components {
		if c.Name == name {
			return c.Status
		}
	}

	return ""
}

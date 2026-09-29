package registry

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPurgePoolDoesNotBlockReadersDuringRebuild pins that PurgePool builds
// the replacement map before taking the package-global poolLock.
//
// cloneInstance allocates a fresh Instance plus its Servers and Topics maps
// for every entry, so a rebuild that runs under the write lock stalls every
// pool reader (GetPool, GetInstance, GetInstances, GetService) for the whole
// pass: a discovery outage on every registry refresh, not a microsecond
// hiccup.
func TestPurgePoolDoesNotBlockReadersDuringRebuild(t *testing.T) {
	// Seed one cheap service so the reader has something to look up. Until
	// the swap lands, "seed" is still the published service; afterwards it
	// is gone, which is how the loop below knows the window closed.
	PurgePool([]*Instance{{ID: uuid.New(), ServiceName: "seed"}})

	// Give each instance real Servers/Topics so cloneInstance does genuine
	// work. This is the cost that used to run under the write lock. The
	// count is sized so the rebuild is long enough (tens of milliseconds)
	// that a reader colliding with the write lock is unmistakable.
	const purgeSize = 100000

	ins := make([]*Instance, 0, purgeSize)

	for range purgeSize {
		in := &Instance{ID: uuid.New(), ServiceName: "bulk"}
		in.Servers = map[string]*Server{
			"grpc": {ID: uuid.New(), InstanceID: in.ID, Type: "grpc", Name: "grpc"},
		}
		in.Topics = map[string]*Topic{
			"events": {Name: "events", Type: "queue", Group: "g"},
		}

		ins = append(ins, in)
	}

	done := make(chan time.Duration, 1)

	go func() {
		start := time.Now()
		PurgePool(ins)
		done <- time.Since(start)
	}()

	// Spin the reader for a bounded window and record the slowest single
	// read. Spinning (rather than one timed read) is what makes this
	// immune to scheduling: against the buggy PurgePool the loop is
	// guaranteed to collide with the write lock at least once, because the
	// rebuild takes orders of magnitude longer than one loop iteration.
	var worst time.Duration

	sawSeed := false

	until := time.Now().Add(10 * time.Second)

	for time.Now().Before(until) {
		start := time.Now()
		svc := GetService("seed")

		if d := time.Since(start); d > worst {
			worst = d
		}

		if svc == nil {
			// The swap landed, so the rebuild window is over.
			break
		}

		sawSeed = true
	}

	purgeTime := <-done

	if !sawSeed {
		t.Fatal("never observed the pool before the swap: the reader never saw the seeded service")
	}

	// Without a rebuild long enough to measure against, the worst-read
	// number proves nothing, so fail loudly instead of passing silently.
	if purgeTime < 10*time.Millisecond {
		t.Fatalf("rebuild took only %v: the fixture is too small to detect reader blocking, raise purgeSize", purgeTime)
	}

	// A read is a map lookup plus a clone of a one-instance service, so it
	// is single-digit microseconds. The threshold is relative rather than
	// absolute because the absolute cost of a rebuild is machine- and
	// size-dependent, but the invariant is not: a reader must never stall
	// for a meaningful fraction of the rebuild. Holding the write lock
	// across the rebuild makes the worst read ~100% of purgeTime.
	if limit := purgeTime / 4; worst > limit {
		t.Fatalf("slowest pool read took %v, more than a quarter of the %v rebuild: a reader was blocked while PurgePool rebuilt, so the write lock is held across the rebuild", worst, purgeTime)
	}
}

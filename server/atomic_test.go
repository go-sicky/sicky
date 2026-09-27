package server

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestAppendAtomicSliceNoLostUpdate: the loop must compare against the
// snapshot it built from, otherwise a concurrent writer's handlers are
// dropped.
func TestAppendAtomicSliceNoLostUpdate(t *testing.T) {
	const (
		writers = 16
		perW    = 64
	)

	var p atomic.Pointer[[]int]

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)

		go func(w int) {
			defer wg.Done()

			for i := range perW {
				AppendAtomicSlice(&p, w*perW+i)
			}
		}(w)
	}

	wg.Wait()

	if p.Load() == nil {
		t.Fatal("nothing published")
	}

	got := *p.Load()
	if len(got) != writers*perW {
		t.Fatalf("published %d values, want %d", len(got), writers*perW)
	}

	seen := make(map[int]bool, len(got))
	for _, v := range got {
		if seen[v] {
			t.Fatalf("value %d published twice", v)
		}

		seen[v] = true
	}
}

func TestAppendAtomicSliceFromEmpty(t *testing.T) {
	var p atomic.Pointer[[]string]
	AppendAtomicSlice(&p, "a", "b")

	if p.Load() == nil {
		t.Fatal("nil pointer left behind")
	}

	got := *p.Load()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v, want [a b]", got)
	}
}

func TestAppendAtomicSliceNilTarget(t *testing.T) {
	// A nil pointer target must be a no-op, not a panic.
	AppendAtomicSlice[string](nil, "a")
}

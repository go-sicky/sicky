package websocket

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCloseIsIdempotent: the reaper and the operator both call Close, and
// a second call used to write another close frame on a connection the
// first call had already torn down.
func TestCloseIsIdempotent(t *testing.T) {
	sess := NewSession(nil)

	if !sess.Valid {
		t.Fatal("a fresh session must be valid")
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	if sess.Valid {
		t.Fatal("Close must mark the session invalid")
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("second close must be a no-op, got %v", err)
	}
}

func TestSetKeyIndexesAndReplaces(t *testing.T) {
	pool := NewPool(10, 60)
	sess := NewSession(nil)
	pool.Put(sess)

	sess.SetKey("first")
	if got := pool.GetByKey("first"); got != sess {
		t.Fatalf("GetByKey(first) = %p, want %p", got, sess)
	}

	sess.SetKey("second")
	if pool.GetByKey("first") != nil {
		t.Fatal("the previous key mapping must be dropped")
	}

	if got := pool.GetByKey("second"); got != sess {
		t.Fatalf("GetByKey(second) = %p, want %p", got, sess)
	}
}

// TestSetKeyDoesNotStealAnotherSession: the old mapping is only dropped
// when this session still owns it - otherwise a key that moved to
// somebody else would be deleted from under them.
func TestSetKeyDoesNotStealAnotherSession(t *testing.T) {
	pool := NewPool(10, 60)
	first := NewSession(nil)
	second := NewSession(nil)
	pool.Put(first)
	pool.Put(second)

	first.SetKey("shared")
	// second takes the key over (last writer wins).
	second.SetKey("shared")

	first.SetKey("rekeyed")
	if got := pool.GetByKey("shared"); got != second {
		t.Fatalf("shared key = %p, want the session that took it over (%p)", got, second)
	}

	if got := pool.GetByKey("rekeyed"); got != first {
		t.Fatalf("rekeyed = %p, want %p", got, first)
	}
}

// TestSessionConcurrentKeyRemoveClose exercises the lock order between
// SetKey (session lock then pool lock), RemoveByID (pool lock then
// session lock) and Close under -race.
func TestSessionConcurrentKeyRemoveClose(t *testing.T) {
	pool := NewPool(10, 60)
	sess := NewSession(nil)
	pool.Put(sess)

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				sess.SetKey(fmt.Sprintf("key-%d", i%4))
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				pool.RemoveByID(sess.ID)
				pool.Put(sess)
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = sess.Close()
				sess.mu.Lock()
				sess.Valid = true
				sess.mu.Unlock()
			}
		}
	})

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

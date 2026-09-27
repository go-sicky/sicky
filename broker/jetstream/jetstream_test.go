package jetstream

import (
	"sync"
	"testing"
	"time"

	"github.com/go-sicky/sicky/broker"
)

// TestPublishRacesWithConnectionWrite guards the handle snapshot: Publish
// used to read brk.conn and brk.streamer without the lock while
// Disconnect cleared both under the lock. The writer below mirrors
// exactly what Disconnect does with those fields.
func TestPublishRacesWithConnectionWrite(t *testing.T) {
	brk := New(&broker.Options{Name: "jetstream-test"}, &Config{})
	if brk == nil {
		t.Fatal("New returned nil (config rejected)")
	}

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = brk.Publish("topic", &broker.Message{Body: []byte("x")})
			}
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				brk.mu.Lock()
				brk.conn = nil
				brk.streamer = nil
				brk.streamInfo = nil
				brk.mu.Unlock()
			}
		}
	})

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

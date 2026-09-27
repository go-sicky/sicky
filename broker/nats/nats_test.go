package nats

import (
	"sync"
	"testing"
	"time"

	"github.com/go-sicky/sicky/broker"
)

// TestPublishRacesWithConnectionWrite guards the handle snapshot: Publish
// used to read brk.conn without the lock while Disconnect cleared it
// under the lock - a data race under the Go memory model. The writer
// below mirrors exactly what Disconnect does with the field.
func TestPublishRacesWithConnectionWrite(t *testing.T) {
	brk := New(&broker.Options{Name: "nats-test"}, &Config{})
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
				brk.mu.Unlock()
			}
		}
	})

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

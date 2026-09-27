package interactive

import (
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/service"
)

// concurrentHandler satisfies Handler for registration tests.
type concurrentHandler struct {
	name  string
	seen  []string
	stops int
}

func (h *concurrentHandler) Name() string { return h.name }

func (h *concurrentHandler) OnInteract(cmd, _ string) error {
	h.seen = append(h.seen, cmd)

	return nil
}

func (h *concurrentHandler) OnStop() error {
	h.stops++

	return nil
}

// TestHandleConcurrentWithDispatch: Handle appended to the live slice
// while the stdin loop walked it in dispatch.
func TestHandleConcurrentWithDispatch(t *testing.T) {
	svc := New(&service.Options{ID: uuid.New(), Name: "interactive-race"}, &Config{})

	stop := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() {
		for range 500 {
			svc.Handle(&concurrentHandler{name: "h"})
			_ = svc.Servers()
		}

		close(stop)
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				svc.dispatch("ping")
				_ = svc.snapshotHandlers()
			}
		}
	})

	wg.Wait()

	if got := len(svc.snapshotHandlers()); got != 500 {
		t.Fatalf("handlers = %d, want 500", got)
	}
}

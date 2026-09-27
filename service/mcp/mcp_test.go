package mcp

import (
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/service/mcp/protocol"
)

// noopHandler is the smallest Handler that compiles: registration is
// what this test exercises, not the handlers themselves.
type noopHandler struct{}

func (h *noopHandler) Name() string        { return "noop" }
func (h *noopHandler) Description() string { return "noop" }

func (h *noopHandler) Tools() []protocol.Tool {
	return nil
}

func (h *noopHandler) CallTool(string, map[string]any) (*protocol.ToolsCallResult, error) {
	return nil, nil
}

func (h *noopHandler) Resources() []protocol.Resource {
	return nil
}

func (h *noopHandler) ReadResource(string) (*protocol.ResourcesReadResult, error) {
	return nil, nil
}

func (h *noopHandler) Prompts() []protocol.Prompt {
	return nil
}

func (h *noopHandler) GetPrompt(string, map[string]string) (*protocol.PromptsGetResult, error) {
	return nil, nil
}

// TestHandlersConcurrentWithRegistration: MCP.Handlers() returned the
// live slice while Handle appended to it - a classic slice-header race.
func TestHandlersConcurrentWithRegistration(t *testing.T) {
	svc := New(&service.Options{ID: uuid.New(), Name: "mcp-race"}, &Config{})
	if svc == nil {
		t.Fatal("New must succeed")
	}

	stop := make(chan struct{})

	var wg sync.WaitGroup

	wg.Go(func() {
		for range 500 {
			svc.Handle(&noopHandler{})

			// The returned slice is a copy: mutating it must not reach
			// the live registry.
			handlers := svc.Handlers()
			if len(handlers) > 0 {
				handlers[0] = nil
			}
		}

		close(stop)
	})

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = svc.Handlers()
				_ = svc.Servers()
				_ = svc.Brokers()
			}
		}
	})

	wg.Wait()

	if len(svc.Handlers()) != 500 {
		t.Fatalf("handlers = %d, want 500 (one was lost or the copy leaked in)", len(svc.Handlers()))
	}
}

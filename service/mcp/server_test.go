package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-sicky/sicky/service/mcp/protocol"
)

// failingHandler fails every call with an error that carries internal
// detail, the way a real tool would.
type failingHandler struct{}

func (failingHandler) Name() string        { return "failing" }
func (failingHandler) Description() string { return "test handler" }

func (failingHandler) Tools() []protocol.Tool {
	return []protocol.Tool{{Name: "boom", Description: "always fails"}}
}

func (failingHandler) CallTool(string, map[string]any) (*protocol.ToolsCallResult, error) {
	return nil, errors.New("query failed: dsn=postgres://app:s3cr3t@db:5432/app")
}

func (failingHandler) Resources() []protocol.Resource {
	return []protocol.Resource{{URI: "secret://x", Name: "x"}}
}

func (failingHandler) ReadResource(string) (*protocol.ResourcesReadResult, error) {
	return nil, errors.New("read failed: /etc/shadow is not a resource")
}

func (failingHandler) Prompts() []protocol.Prompt {
	return []protocol.Prompt{{Name: "p"}}
}

func (failingHandler) GetPrompt(string, map[string]string) (*protocol.PromptsGetResult, error) {
	return nil, errors.New("prompt failed: credential=abc123")
}

func newTestServer() *MCPServer {
	return NewMCPServer(
		protocol.ImplementationInfo{Name: "test", Version: "1.0.0"},
		protocol.ServerCapabilities{},
	)
}

func parseRequest(t *testing.T, raw string) *protocol.Request {
	t.Helper()

	req, err := protocol.ParseRequest([]byte(raw))
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}

	return req
}

// TestDispatchRequiresInitialize: without the handshake every API method
// would be served to a stream that never introduced itself.
func TestDispatchRequiresInitialize(t *testing.T) {
	s := newTestServer()
	ctx := context.Background()

	resp := s.dispatchRequest(ctx, parseRequest(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp == nil || resp.Error == nil || resp.Error.Code != protocol.ErrCodeInvalidRequest {
		t.Fatalf("tools/list before initialize = %+v, want ErrCodeInvalidRequest", resp)
	}

	// Ping is allowed before the handshake (liveness check).
	resp = s.dispatchRequest(ctx, parseRequest(t, `{"jsonrpc":"2.0","id":2,"method":"ping"}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("ping before initialize = %+v, want success", resp)
	}

	// Run the handshake.
	resp = s.dispatchRequest(ctx, parseRequest(t, `{"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"c","version":"1"}}}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize = %+v, want success", resp)
	}

	s.dispatchNotification(ctx, &protocol.Notification{Method: protocol.MethodInitialized})
	if !s.isInitialized() {
		t.Fatal("initialized notification must open the gate")
	}

	resp = s.dispatchRequest(ctx, parseRequest(t, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`))
	if resp == nil || resp.Error != nil {
		t.Fatalf("tools/list after initialize = %+v, want success", resp)
	}
}

// TestHandlerErrorsAreMasked: tool errors used to reach the client
// verbatim (`err.Error()`), handing over DSNs, paths and credentials.
func TestHandlerErrorsAreMasked(t *testing.T) {
	s := newTestServer()
	s.Handle(failingHandler{})
	ctx := context.Background()

	// Handshake first so the gate is open.
	_ = s.dispatchRequest(ctx, parseRequest(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"c","version":"1"}}}`))
	s.dispatchNotification(ctx, &protocol.Notification{Method: protocol.MethodInitialized})

	secrets := []string{"s3cr3t", "postgres", "/etc/shadow", "abc123"}
	calls := []string{
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"boom","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"secret://x"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/get","params":{"name":"p"}}`,
	}

	for _, call := range calls {
		resp := s.dispatchRequest(ctx, parseRequest(t, call))
		if resp == nil || resp.Error == nil || resp.Error.Code != protocol.ErrCodeInternalError {
			t.Fatalf("%s = %+v, want an internal error", call, resp)
		}

		for _, secret := range secrets {
			if strings.Contains(resp.Error.Message, secret) {
				t.Errorf("%s leaked %q: %q", call, secret, resp.Error.Message)
			}
		}

		if resp.Error.Message != "internal error" {
			t.Errorf("%s message = %q, want the generic text", call, resp.Error.Message)
		}
	}
}

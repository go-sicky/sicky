/**
 * @file closedconn_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

// Both spellings are load-bearing: net/http wraps the condition so errors.Is
// matches, while fasthttp only produces the text. A filter that handled one
// would still report a failed shutdown for the other stack.
func TestIsClosedConnError(t *testing.T) {
	// The wrapped form, which is what net/http produces.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	if cerr := listener.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}

	// Re-closing a real listener is the canonical source.
	if !IsClosedConnError(listener.Close()) {
		t.Error("re-closing a listener must be recognized as benign double-close")
	}

	if !IsClosedConnError(fmt.Errorf("shutdown: %w", net.ErrClosed)) {
		t.Error("net.ErrClosed must be recognized, including wrapped")
	}

	// The bare text form, which is what fasthttp produces.
	if !IsClosedConnError(errors.New("close tcp 127.0.0.1:1234: use of closed network connection")) {
		t.Error("the fasthttp message must be recognized as text")
	}

	// A real failure must not be filtered away: this is the whole point.
	for _, err := range []error{
		nil,
		errors.New("context deadline exceeded"),
		errors.New("address already in use"),
		fmt.Errorf("listen tcp: %w", errors.New("permission denied")),
	} {
		if IsClosedConnError(err) {
			t.Errorf("IsClosedConnError(%v) = true; a real failure must reach the caller", err)
		}
	}
}

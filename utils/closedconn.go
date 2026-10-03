/**
 * @file net.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import (
	"errors"
	"net"
	"strings"
)

// IsClosedConnError reports benign double-close noise.
//
// Both server stacks close their listener twice on purpose: once through the
// framework's own shutdown and once as a backstop, because a shutdown racing
// the serve goroutine's registration can leave the framework holding a stale
// listener entry. The second close fails with "use of closed network
// connection", which says nothing about whether the socket is down — it is.
//
// Callers use this to keep such an error out of a Stop return value, which
// callers treat as "the server did not shut down" and which a supervisor will
// read as a failed shutdown.
//
// Both spellings matter: net/http wraps the condition so that errors.Is
// matches, while fasthttp's own message has to be matched as text.
func IsClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	return strings.Contains(err.Error(), "use of closed network connection")
}

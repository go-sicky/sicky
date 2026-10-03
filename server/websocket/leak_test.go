/**
 * @file leak_test.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package websocket

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for the whole package. The
// websocket server owns an accept loop, a session pool and an idle reaper, and
// none of them is joined by a Stop that gives up on its shutdown timeout.
//
// The fasthttp exemptions that leakcheck.Options carries come from this
// package's own test helper: TestCheckOrigin builds a fasthttp.RequestCtx by
// hand to exercise the fiber-based Origin check, and constructing one starts
// goroutines with no public stop. The server under test uses gobwas/ws
// directly and starts neither.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

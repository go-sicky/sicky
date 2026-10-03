/**
 * @file leak_test.go
 * @package protocol
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package protocol

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The HTTP transport keeps a per-connection SSE pump alive; without a leak check a dropped subscriber loop is invisible, because the client simply never receives.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

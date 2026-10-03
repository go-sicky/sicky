/**
 * @file leak_test.go
 * @package tcp
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package tcp

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The TCP server owns an accept loop, a connection set and an idle reaper; a Stop that fails to join any of them leaves them running for the life of the process.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

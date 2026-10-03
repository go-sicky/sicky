/**
 * @file leak_test.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package consul

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The consul watcher starts a long-running modify loop. It is the one goroutine the registry cannot function without, and the one most easily orphaned.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

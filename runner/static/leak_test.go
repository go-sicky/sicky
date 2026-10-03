/**
 * @file leak_test.go
 * @package static
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package static

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The runner keeps a pool of worker goroutines; Start/Stop must leave none behind on either path.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

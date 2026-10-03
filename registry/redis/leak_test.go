/**
 * @file leak_test.go
 * @package redis
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package redis

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The redis registry starts a watch loop goroutine that reconnects on its own schedule, so nothing else in the package would ever notice it outliving Stop.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

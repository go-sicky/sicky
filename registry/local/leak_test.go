/**
 * @file leak_test.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package local

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The file-backed registry owns an fsnotify watcher goroutine plus its own debounce timer.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

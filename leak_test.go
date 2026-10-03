/**
 * @file leak_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs a goroutine-leak check for this package. Run starts the purge ticker, the SIGHUP reload goroutine and every subsystem, and joins only some of them before shutdown returns.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreCurrent(),
	)
}

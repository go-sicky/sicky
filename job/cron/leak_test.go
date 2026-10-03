/**
 * @file leak_test.go
 * @package cron
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package cron

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The cron job owns the gocron scheduler goroutine.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

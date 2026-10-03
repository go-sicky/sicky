/**
 * @file leak_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for the whole package.
// WaitGroupTimeout and WaitTimeout each start a goroutine that is meant to be
// abandoned when its deadline expires — that abandonment is the feature. The
// check makes sure the abandoned goroutine still exits shortly after, instead
// of parking forever on a Wait that nobody will ever complete.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

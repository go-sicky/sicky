/**
 * @file leak_test.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package http

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for the whole package. The HTTP
// server owns an accept loop and the listener it was handed; a Stop that
// returns without joining either leaves them serving for the life of the
// process, and nothing in the suite could see that before.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

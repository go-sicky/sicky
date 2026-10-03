/**
 * @file leak_test.go
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package fiber

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for the whole package. The fiber
// server serves on its own listener and joins the serve goroutines through
// srv.wg; a Stop that returns early or times out leaves them serving.
//
// This package sits directly on fasthttp, so the fasthttp exemptions in
// leakcheck.Options apply to the production path here, not only to test
// helpers. See that package for why they are unavoidable.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

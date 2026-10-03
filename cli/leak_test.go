/**
 * @file leak_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package cli

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for the whole package. The CLI
// starts two long-lived goroutines: the MCP server's accept loop and the
// --watch child supervisor, which kills a process group.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

/**
 * @file leak_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package grpc

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// TestMain installs a goroutine-leak check for this package. The gRPC client runs a discovery watcher plus a 30s resync ticker in Service mode, both of which are stopped only through a done channel that Disconnect closes.
//
// IgnoreCurrent covers what testing and the runtime already had running when
// the suite started. Anything a test starts must be gone by the end of the
// package run.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, leakcheck.Options()...)
}

/**
 * @file errors_test.go
 * @package server_test
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package server_test

import (
	"errors"
	"strings"
	"testing"

	cltGRPC "github.com/go-sicky/sicky/client/grpc"
	"github.com/go-sicky/sicky/server"
	srvFiber "github.com/go-sicky/sicky/server/fiber"
	srvGRPC "github.com/go-sicky/sicky/server/grpc"
	srvHTTP "github.com/go-sicky/sicky/server/http"
	srvWS "github.com/go-sicky/sicky/server/websocket"
)

// Five implementations used to each declare their own ErrIncompleteTLSConfig
// with errors.New, which allocates a distinct value per call site. Matching
// the condition therefore required one errors.Is arm per sub-package, and a
// caller holding only a server.Server had no single target to match: the
// condition is identical in all five, but no two values were ever equal.
func TestIncompleteTLSConfigIsOneValueAcrossImplementations(t *testing.T) {
	aliases := map[string]error{
		"server":           server.ErrIncompleteTLSConfig,
		"server/http":      srvHTTP.ErrIncompleteTLSConfig,
		"server/grpc":      srvGRPC.ErrIncompleteTLSConfig,
		"server/fiber":     srvFiber.ErrIncompleteTLSConfig,
		"server/websocket": srvWS.ErrIncompleteTLSConfig,
		"client/grpc":      cltGRPC.ErrIncompleteTLSConfig,
	}

	for from, err := range aliases {
		if err == nil {
			t.Fatalf("%s: sentinel must not be nil", from)
		}

		for to, other := range aliases {
			if from == to {
				continue
			}

			if !errors.Is(err, other) {
				t.Errorf("errors.Is(%s, %s) = false, want true: the aliases are distinct values", from, to)
			}
		}
	}
}

// A caller holding only the abstraction must be able to match without
// importing any implementation — that is the point of the shared sentinel.
func TestIncompleteTLSConfigMatchesThroughTheAbstraction(t *testing.T) {
	err := errors.Join(server.ErrIncompleteTLSConfig)

	if !errors.Is(err, server.ErrIncompleteTLSConfig) {
		t.Fatal("the shared sentinel must match itself")
	}

	for from, impl := range map[string]error{
		"http":      srvHTTP.ErrIncompleteTLSConfig,
		"grpc":      srvGRPC.ErrIncompleteTLSConfig,
		"fiber":     srvFiber.ErrIncompleteTLSConfig,
		"websocket": srvWS.ErrIncompleteTLSConfig,
		"client":    cltGRPC.ErrIncompleteTLSConfig,
	} {
		if !errors.Is(impl, server.ErrIncompleteTLSConfig) {
			t.Errorf("%s: does not match server.ErrIncompleteTLSConfig", from)
		}
	}
}

// The message is what an operator reads in a startup log, so pinning it keeps
// the aliases honest: an implementation that kept its own errors.New would
// still pass the identity checks above only if the string also matched.
func TestIncompleteTLSConfigMessageNamesBothFields(t *testing.T) {
	msg := server.ErrIncompleteTLSConfig.Error()

	for _, want := range []string{"tls_cert_pem", "tls_key_pem"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q must name %q so the operator knows which field to fix", msg, want)
		}
	}
}

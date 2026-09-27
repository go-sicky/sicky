package fiber

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
)

// TestRequestLabelsBoundedForUnmatchedRequests guards against the
// cardinality bomb: fiber synthesizes a route out of the raw request
// path when nothing matched, so `/aaaa0`, `/aaaa1`, ... used to become
// one time series each.
func TestRequestLabelsBoundedForUnmatchedRequests(t *testing.T) {
	srv := New(
		&server.Options{Name: "fiber-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)

	srv.App().Get("/ping", func(c fiber.Ctx) error {
		return nil
	})

	before := testutil.CollectAndCount(metrics.ServerRequestsTotal)

	// Client-minted methods on client-minted paths.
	for i := range 50 {
		testResponse(t, srv, fmt.Sprintf("GET%d", i), fmt.Sprintf("/nope%d", i))
	}

	// Legitimate method, still unmatched paths.
	for i := range 50 {
		testResponse(t, srv, http.MethodGet, fmt.Sprintf("/nope%d", i))
	}

	// Bogus method on a registered route: the template stays verbatim,
	// the method collapses.
	testResponse(t, srv, "GETX", "/ping")

	// Expected new series: (OTHER, unmatched, 404), (GET, unmatched, 404)
	// and (OTHER, /ping, 405).
	delta := testutil.CollectAndCount(metrics.ServerRequestsTotal) - before
	if delta > 3 {
		t.Fatalf("101 requests created %d new time series, want at most 3", delta)
	}
}

// testResponse issues a request through the full middleware chain,
// drains the body and closes it, returning the final status.
func testResponse(t *testing.T, srv *FiberServer, method, path string) int {
	t.Helper()

	resp, err := srv.App().Test(httptest.NewRequest(method, path, http.NoBody))
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode
}

// TestRouteLabelKeepsRegisteredTemplate: matched routes keep full
// fidelity - only the synthetic fallback collapses.
func TestRouteLabelKeepsRegisteredTemplate(t *testing.T) {
	srv := New(
		&server.Options{Name: "fiber-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)

	var matched, unmatched string
	srv.App().Use(func(c fiber.Ctx) error {
		err := c.Next()
		if matched == "" {
			matched = routeLabel(c)
		} else if unmatched == "" {
			unmatched = routeLabel(c)
		}

		return err
	})
	srv.App().Get("/user/:id", func(c fiber.Ctx) error {
		return nil
	})

	testResponse(t, srv, http.MethodGet, "/user/42")
	testResponse(t, srv, http.MethodGet, "/does-not-exist")

	if matched != "/user/:id" {
		t.Fatalf("matched route label = %q, want /user/:id", matched)
	}
	if unmatched != metrics.UnmatchedRoute {
		t.Fatalf("unmatched route label = %q, want %q", unmatched, metrics.UnmatchedRoute)
	}
}

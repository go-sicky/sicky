package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/uptrace/bunrouter"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
)

// TestRequestLabelsBoundedForUnmatchedRequests guards against the
// cardinality bomb: the method and the unmatched route are client input,
// so a loop of `GET0 /nope0`, `GET1 /nope1`, ... must collapse into a
// handful of series instead of minting two per request.
func TestRequestLabelsBoundedForUnmatchedRequests(t *testing.T) {
	srv := New(
		&server.Options{Name: "http-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)

	srv.router.GET("/ping", func(w http.ResponseWriter, r bunrouter.Request) error {
		return nil
	})

	before := testutil.CollectAndCount(metrics.ServerRequestsTotal)

	// Client-minted methods on client-minted paths.
	for i := range 50 {
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, httptest.NewRequest(fmt.Sprintf("GET%d", i), fmt.Sprintf("/nope%d", i), http.NoBody))
	}

	// Legitimate method, still unmatched paths.
	for i := range 50 {
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/nope%d", i), http.NoBody))
	}

	// Bogus method on a registered route: the template stays verbatim,
	// the method collapses.
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest("GETX", "/ping", http.NoBody))

	// Expected new series: (OTHER, unmatched, 404), (GET, unmatched, 404)
	// and (OTHER, /ping, 405).
	delta := testutil.CollectAndCount(metrics.ServerRequestsTotal) - before
	if delta > 3 {
		t.Fatalf("101 requests created %d new time series, want at most 3", delta)
	}
}

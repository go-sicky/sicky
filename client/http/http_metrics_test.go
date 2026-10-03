/**
 * @file http_metrics_test.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/metrics"
)

// A gateway or webhook relay passes a caller-supplied URL into Do, so both
// the method and the host that reach the metric labels are attacker-
// influenced. These were the only two label call sites in the tree that skipped
// normalization, so one request minted one permanent Prometheus series: a
// counter child, a histogram child, a cache entry and a /metrics line, none of
// which are ever released.
//
// Requests are routed through a DialFunc so req.URL.Host can keep the hostile
// value while the connection still reaches the test server. Host is what the
// normalizer reads, so this exercises the real path.
func TestDoNormalizesCallerSuppliedLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	srvAddr := strings.TrimPrefix(srv.URL, "http://")
	dialTo := func(string) (string, error) { return srvAddr, nil }

	clt := New(&client.Options{ID: uuid.New(), Name: "metrics-test"}, &Config{})

	// Host: three distinct destinations, one method. All must collapse.
	const hosts = 3

	hostSeries := seriesCounter(t, metrics.UnknownMethod)
	hostRaw := seriesCounter(t, "PROPFIND")

	for _, h := range []string{"a.example.com", "b.example.com", "c.example.com"} {
		do(t, clt, dialTo, "PROPFIND", h)
	}

	if got := hostSeries.delta(); got != float64(hosts) {
		t.Errorf("collapsed host series counted %v, want %d", got, hosts)
	}

	if got := hostRaw.delta(); got != 0 {
		t.Errorf("raw host created its own series (%v observations)", got)
	}

	// Method: a separate axis, normalized independently.
	for _, m := range []struct{ attempt, into string }{
		{"get", "GET"},
		{"X-CUSTOM", metrics.UnknownMethod},
	} {
		// Snapshot both series first: measuring after the request would make
		// every delta zero and the assertion vacuous.
		into := seriesCounter(t, m.into)
		raw := seriesCounter(t, m.attempt)

		do(t, clt, dialTo, m.attempt, "d.example.com")

		if into.delta() != 1 {
			t.Errorf("%q landed %v times in the %q series, want 1", m.attempt, into.delta(), m.into)
		}

		if raw.delta() != 0 {
			t.Errorf("%q created its own series (%v observations): it was not normalized", m.attempt, raw.delta())
		}
	}

	// A standard method keeps its own series verbatim — normalization must not
	// collapse GET into OTHER.
	get := seriesCounter(t, http.MethodGet)
	other := seriesCounter(t, metrics.UnknownMethod)

	do(t, clt, dialTo, http.MethodGet, "e.example.com")

	if got := get.delta(); got != 1 {
		t.Errorf("GET series counted %v, want 1: a standard method must stay verbatim", got)
	}

	if got := other.delta(); got != 0 {
		t.Errorf("GET also landed %v times in OTHER: the normalizer is not case-exact", got)
	}
}

func do(t *testing.T, clt *HTTPClient, dial DialFunc, method, host string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, "http://"+host+"/x", http.NoBody)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, host, err)
	}

	resp, err := clt.DoWithTransport(req, dial)
	if err != nil {
		t.Fatalf("Do(%s %s): %v", method, host, err)
	}

	_ = resp.Body.Close()
}

// clientSeries is a snapshot handle: it records the current value so delta()
// reports what the code under test added, rather than what earlier tests in the
// same binary left behind.
type clientSeries struct {
	before float64
	method string
}

func seriesCounter(t *testing.T, method string) clientSeries {
	t.Helper()

	// Label order is (client, method, host, code).
	c, err := metrics.ClientRequestsTotal.GetMetricWithLabelValues("http", method, metrics.OtherHost, "200")
	if err != nil {
		t.Fatalf("get metric for method %q: %v", method, err)
	}

	return clientSeries{before: testutil.ToFloat64(c), method: method}
}

func (s clientSeries) delta() float64 {
	c, err := metrics.ClientRequestsTotal.GetMetricWithLabelValues("http", s.method, metrics.OtherHost, "200")
	if err != nil {
		return 0
	}

	return testutil.ToFloat64(c) - s.before
}

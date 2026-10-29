/**
 * @file rollback_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/metrics"
)

func TestRollbackS3ClientsResetsMetrics(t *testing.T) {
	metrics.SetInfraUp("s3:a", true)
	metrics.SetInfraUp("s3:b", true)

	rollbackS3Clients(map[string]*s3.Client{
		"a": new(s3.Client),
		"b": nil,
	})

	for _, name := range []string{"s3:a", "s3:b"} {
		if got := testutil.ToFloat64(metrics.InfraUp.WithLabelValues(name)); got != 0 {
			t.Fatalf("sicky_infra_up{infra=%q} = %v, want 0", name, got)
		}
	}
}

func TestRollbackRistrettoClientsClosesAndResets(t *testing.T) {
	defaultCache, err := newRistrettoClient(&RistrettoConfig{
		NumCounters: 100,
		MaxCost:     100,
		BufferItems: 64,
	})
	if err != nil {
		t.Fatalf("newRistrettoClient: %v", err)
	}

	named, err := newRistrettoClient(&RistrettoConfig{
		NumCounters: 100,
		MaxCost:     100,
		BufferItems: 64,
	})
	if err != nil {
		defaultCache.Close()
		t.Fatalf("newRistrettoClient: %v", err)
	}

	metrics.SetInfraUp("ristretto:cache", true)
	rollbackRistrettoClients(defaultCache, map[string]*ristretto.Cache[string, any]{"cache": named})

	if got := testutil.ToFloat64(metrics.InfraUp.WithLabelValues("ristretto:cache")); got != 0 {
		t.Fatalf("sicky_infra_up{infra=ristretto:cache} = %v, want 0", got)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

package sicky

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/service"
)

// TestRunCountsSuccessfulServiceStarts: the "ok" branch used to build the
// label values but never called Inc(), so sickey_service_starts_total
// only ever counted failures.
func TestRunCountsSuccessfulServiceStarts(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Clear()
	service.Set(svc)

	label := svc.String()
	before := testutil.ToFloat64(metrics.ServiceStartsTotal.WithLabelValues(label, "ok"))

	AfterStart(func(context.Context) error {
		cancel()

		return nil
	})

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	after := testutil.ToFloat64(metrics.ServiceStartsTotal.WithLabelValues(label, "ok"))
	if after-before != 1 {
		t.Fatalf("sicky_service_starts_total{result=ok} delta = %v, want 1", after-before)
	}
}

/**
 * @file run_infra_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package sicky

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/service"
)

// Run must close named infra instances on shutdown, not only the shared
// singleton. Ristretto is used because it starts locally.
func TestRunShutsDownNamedInfraInstances(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	_ = infra.ShutdownRistretto()
	t.Cleanup(func() { _ = infra.ShutdownRistretto() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options = testOptions()
	options.Context = ctx
	options.MustInfra = []string{"ristretto"}

	svc := &fakeService{opts: &service.Options{ID: uuid.New(), Name: "fake"}}
	service.Clear()
	service.Set(svc)

	AfterStart(func(context.Context) error {
		cancel()

		return nil
	})

	err := Run(&Config{
		Infra: &InfraConfig{
			Ristretto: &infra.RistrettoConfig{
				NoDefault: true,
				Instances: map[string]*infra.RistrettoConfig{
					"hot": {NumCounters: 100, MaxCost: 100, BufferItems: 64},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}

	if infra.GetRistretto("hot") != nil {
		t.Fatal("Run must shut down every named infra instance")
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

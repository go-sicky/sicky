/**
 * @file must_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"context"
	"testing"
	"time"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/service"
)

// MustInfra is exported, so anything outside this package may read it — a
// /debug handler, an operator's script, a second goroutine wiring services.
// Run used to flip one entry per component as it initialized them, which put
// those readers in a concurrent read/write on a map. The runtime does not
// recover from that: it is `fatal error: concurrent map read and map write`,
// and the process dies without a stack pointing at the offending code.
//
// The accessors below are the supported read path. This test runs them against
// a live Run so the race detector sees it; under `make verify` that is a hard
// failure, and the fatal would be one lost process in production.
func TestMustSnapshotIsSafeWhileRunProceeds(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	// Ristretto is in-process, so it initializes for real without a network
	// or a server — which is what makes consumeMustInfra actually run. A
	// requirement nothing initializes would leave the write path untouched
	// and the test would prove nothing.
	//
	// Run blocks until its context is canceled once every requirement is
	// met, so the test supplies one.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	options = testOptions()
	options.Context = ctx
	options.MustInfra = []string{"ristretto"}
	service.Clear()

	stop := make(chan struct{})

	// A reader that behaves like a diagnostics endpoint: poll the whole set
	// as fast as it can for the whole of Run.
	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			select {
			case <-stop:
				return
			default:
			}

			_ = MustInfraSnapshot()
			_ = MustBrokerRequired()
			_ = MustRegistryRequired()

			// Throttle: the snapshot allocates a map per call, and an
			// unthrottled loop under -race starves Run rather than racing it.
			time.Sleep(50 * time.Microsecond)
		}
	}()

	cfg := &Config{
		Manager: &ManagerConfig{Enable: new(false)},
		Infra:   &InfraConfig{Ristretto: &infra.RistrettoConfig{}},
	}

	if err := Run(cfg); err != nil {
		close(stop)
		<-done
		t.Fatalf("run: %v", err)
	}

	close(stop)
	<-done

	if MustInfraSnapshot()[componentRistretto] {
		t.Error("the requirement must be consumed once the component is initialized")
	}
}

// The snapshot must be the caller's own copy: mutating it cannot corrupt the
// framework's state, and the alternative would make the accessor useless for
// the callers it exists for.
func TestMustInfraSnapshotIsACopy(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	publishMustInfra(map[string]bool{componentRedis: true})

	got := MustInfraSnapshot()
	if !got[componentRedis] {
		t.Fatal("the snapshot lost a published requirement")
	}

	got[componentRedis] = false
	delete(got, componentRedis)

	again := MustInfraSnapshot()
	if !again[componentRedis] {
		t.Error("mutating the snapshot changed the framework's state")
	}
}

// A nil set must read back as an empty one, not as nil: callers range over it
// and a nil map would make len() the only usable answer.
func TestMustInfraSnapshotHandlesEmpty(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	publishMustInfra(nil)

	if got := MustInfraSnapshot(); got == nil {
		t.Fatal("MustInfraSnapshot returned nil")
	} else if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// Unknown names in Options.MustInfra must be dropped, not added as keys. A
// typo that silently became a requirement would fail startup with a message
// naming a component no code has heard of.
func TestMustInfraPendingDropsUnknownComponents(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	var warned []string

	pending := mustInfraPending(
		[]string{"redis", "  ", "not-a-component", "BUN "},
		func(name string) { warned = append(warned, name) },
	)

	if !pending[componentRedis] {
		t.Error("redis must be required")
	}

	if !pending[componentBun] {
		t.Error("a component name is lowercased and trimmed before lookup")
	}

	if _, ok := pending["not-a-component"]; ok {
		t.Error("an unknown component must not become a requirement")
	}

	if len(warned) != 1 || warned[0] != "not-a-component" {
		t.Errorf("warned = %v, want exactly the unknown name (the blank entry is not a typo)", warned)
	}
}

// Consuming a requirement that was never set must be a no-op, not a publish
// that drops other pending entries.
func TestConsumeMustInfraIgnoresUnrequested(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	publishMustInfra(map[string]bool{componentRedis: true})

	pending := mustInfraPending(nil, func(string) {})

	consumeMustInfra(pending, componentS3)

	if !MustInfraSnapshot()[componentRedis] {
		t.Error("consuming an unrequested component dropped a live requirement")
	}

	if MustInfraSnapshot()[componentS3] {
		t.Error("consuming an unrequested component added one")
	}
}

// Run must not silently skip the broker requirement on a second run: the
// exported flag is cleared by the first run's startup checks, and the second
// run re-derives it from Options. That behavior is what the private pending
// copy has to preserve.
func TestMustFlagsAreRederivedOnSecondRun(t *testing.T) {
	restore := snapshotGlobals()
	defer restore()

	options = testOptions()
	options.MustBroker = true
	service.Clear()

	cfg := &Config{Manager: &ManagerConfig{Enable: new(false)}}

	err := Run(cfg)
	if err == nil {
		t.Fatal("a required broker with none configured must fail startup")
	}

	if !MustBrokerRequired() {
		t.Error("the requirement must survive a failed startup so the error is not hidden on a retry")
	}
}

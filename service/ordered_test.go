package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
)

// namedService is a minimal Service whose Options().Name is the tag a test
// asserts on. fakeService hardcodes the name "fake", which would make every
// service indistinguishable in an ordering assertion.
type namedService struct{ opts *Options }

func (n *namedService) Context() context.Context { return context.Background() }

func (n *namedService) Options() *Options { return n.opts }

func (n *namedService) String() string { return n.opts.Name }

func (n *namedService) Start() []error { return nil }

func (n *namedService) Stop() []error { return nil }

func (n *namedService) Servers(...server.Server) []server.Server { return nil }

func (n *namedService) Brokers(...broker.Broker) []broker.Broker { return nil }

func (n *namedService) Jobs(...job.Job) []job.Job { return nil }

func (n *namedService) Registries(...registry.Registry) []registry.Registry { return nil }

func (n *namedService) Tracers(...tracer.Tracer) []tracer.Tracer { return nil }

func newNamed(name string) *namedService {
	return &namedService{opts: &Options{ID: uuid.New(), Name: name}}
}

// orderNames renders the Ordered result as the service names, so a failure
// prints the actual sequence instead of opaque pointers.
func orderNames(svcs []Service) []string {
	out := make([]string, 0, len(svcs))
	for _, svc := range svcs {
		out = append(out, svc.Options().Name)
	}

	return out
}

func sameSeq(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func TestOrderedFollowsRegistrationSequence(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	// One Set per service, deliberately not variadic: the order of the Set
	// calls is the contract, so the test states it call by call.
	Set(newNamed("a"))
	Set(newNamed("b"))
	Set(newNamed("c"))

	got := orderNames(Ordered())
	want := []string{"a", "b", "c"}

	if !sameSeq(got, want) {
		t.Fatalf("Ordered = %v, want %v: registration order is the start/stop contract", got, want)
	}
}

func TestOrderedIsStableAcrossRepeatedCalls(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	Set(newNamed("a"), newNamed("b"), newNamed("c"))

	// Go randomizes map iteration on every range, so a map-backed Ordered
	// would disagree with itself between two calls over the same registry.
	first := orderNames(Ordered())

	for range 20 {
		if got := orderNames(Ordered()); !sameSeq(got, first) {
			t.Fatalf("Ordered returned %v then %v: the sequence is not stable", first, got)
		}
	}

	if !sameSeq(first, []string{"a", "b", "c"}) {
		t.Fatalf("Ordered = %v, want [a b c]", first)
	}
}

func TestOrderedIgnoresRejectedRegistrations(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	first := newNamed("a")
	second := newNamed("b")

	Set(first, second)

	// Same ID as `second`: Set skips it, so it must not take a sequence slot
	// and must not push the next registration later. A rejected
	// registration that consumed a sequence number would leave a gap that
	// grows with every retry.
	Set(&namedService{opts: second.Options()}, newNamed("c"))

	got := orderNames(Ordered())
	want := []string{"a", "b", "c"}

	if !sameSeq(got, want) {
		t.Fatalf("Ordered = %v, want %v: a rejected re-registration must not move anything", got, want)
	}
}

func TestOrderedSkipsNilServices(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	Set(newNamed("a"), nil, newNamed("b"))

	got := orderNames(Ordered())
	want := []string{"a", "b"}

	if !sameSeq(got, want) {
		t.Fatalf("Ordered = %v, want %v: a nil service is skipped, not sequenced", got, want)
	}
}

func TestClearRenumbersTheSequence(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	Set(newNamed("old"))

	// A restart clears the registry but must not leave the counter running:
	// a service registered after the Clear would otherwise sort ahead of
	// one registered before it.
	Clear()

	Set(newNamed("new1"), newNamed("new2"))

	got := orderNames(Ordered())
	want := []string{"new1", "new2"}

	if !sameSeq(got, want) {
		t.Fatalf("Ordered after Clear = %v, want %v", got, want)
	}
}

func TestOrderedReturnsACopy(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	Set(newNamed("a"))

	first := Ordered()
	if len(first) != 1 {
		t.Fatalf("Ordered length = %d, want 1", len(first))
	}

	first[0] = nil

	if second := Ordered(); len(second) != 1 || second[0] == nil {
		t.Fatal("mutating the Ordered result must not corrupt the registry")
	}
}

/**
 * @file discovery_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 10/24/2026
 */

package grpc

import (
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"

	"github.com/go-sicky/sicky/registry"
)

// drainPoolNotify empties the shared pool notification channel so a watcher
// under test only observes events this test sends. The channel is buffered
// and stable across InitPool, so a leftover event from an earlier test would
// otherwise be consumed as if it were this test's update.
func drainPoolNotify(pool *registry.Pool) {
	for {
		select {
		case <-pool.Notify:
		default:
			return
		}
	}
}

// stateAddrs extracts the address strings of a resolver state, sorted so
// assertions are order-independent.
func stateAddrs(state resolver.State) []string {
	addrs := make([]string, 0, len(state.Addresses))
	for _, addr := range state.Addresses {
		addrs = append(addrs, addr.Addr)
	}

	slices.Sort(addrs)

	return addrs
}

// recordingResolverCC is a resolver.ClientConn that taps state updates. It
// never blocks the resolver: the tap is diagnostic, not a synchronization
// point for the product code.
//
// The embedded interface (left nil) satisfies the deprecated NewAddress and
// ParseServiceConfig methods, which the manual resolver never calls; the
// resolver.ClientConn documentation recommends exactly this for test fakes
// so a future method addition does not break the fake.
type recordingResolverCC struct {
	resolver.ClientConn

	updates chan resolver.State

	mu   sync.Mutex
	last resolver.State
	errs []error
}

func newRecordingResolverCC() *recordingResolverCC {
	return &recordingResolverCC{updates: make(chan resolver.State, 8)}
}

func (cc *recordingResolverCC) UpdateState(state resolver.State) error {
	cc.mu.Lock()
	cc.last = state
	cc.mu.Unlock()

	select {
	case cc.updates <- state:
	default:
	}

	return nil
}

func (cc *recordingResolverCC) ReportError(err error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.errs = append(cc.errs, err)
}

func (cc *recordingResolverCC) lastState() resolver.State {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	return cc.last
}

func waitForResolverState(t *testing.T, cc *recordingResolverCC, want []string) {
	t.Helper()

	want = slices.Clone(want)
	slices.Sort(want)

	timeout := time.After(5 * time.Second)
	for {
		select {
		case state := <-cc.updates:
			if slices.Equal(stateAddrs(state), want) {
				return
			}
		case <-timeout:
			t.Fatalf("resolver never received %v; last state %v", want, stateAddrs(cc.lastState()))
		}
	}
}

// TestResolveGRPCAddrs pins the pool-to-address translation used by Service
// mode: only grpc servers, advertised address precedence, and every skip
// condition. Order is deliberately not asserted - the pool is a map.
func TestResolveGRPCAddrs(t *testing.T) {
	registry.InitPool()
	t.Cleanup(func() {
		registry.InitPool()
	})

	const service = "resolve-test-svc"

	// A nil instance entry is accepted by the pool (clone keeps the nil) and
	// must be skipped rather than dereferenced.
	registry.RegisterService(&registry.Service{
		Service: service,
		Instances: map[uuid.UUID]*registry.Instance{
			uuid.Nil: nil,
		},
	})

	register := func(ins *registry.Instance) {
		ins.ServiceName = service
		registry.RegisterInstance(ins)
	}

	// Server advertise address wins over the instance address, and a
	// non-grpc server on the same instance is filtered out.
	register(&registry.Instance{
		ID:               uuid.New(),
		AdvertiseAddress: "10.0.0.1",
		Servers: map[string]*registry.Server{
			"grpc": {
				Type:             "grpc",
				AdvertiseAddress: "10.0.0.10",
				Port:             9000,
			},
			"http": {
				Type:             "http",
				AdvertiseAddress: "10.0.0.11",
				Port:             8080,
			},
		},
	})

	// No server address: the instance advertise address is the fallback.
	register(&registry.Instance{
		ID:               uuid.New(),
		AdvertiseAddress: "10.0.0.2",
		Servers: map[string]*registry.Server{
			"grpc": {
				Type: "grpc",
				Port: 9001,
			},
		},
	})

	// Neither address: skipped, even with a usable port.
	register(&registry.Instance{
		ID: uuid.New(),
		Servers: map[string]*registry.Server{
			"grpc": {
				Type: "grpc",
				Port: 9002,
			},
		},
	})

	// grpc entries with port <= 0 are skipped.
	register(&registry.Instance{
		ID:               uuid.New(),
		AdvertiseAddress: "10.0.0.5",
		Servers: map[string]*registry.Server{
			"zero": {
				Type: "grpc",
				Port: 0,
			},
			"negative": {
				Type: "grpc",
				Port: -1,
			},
		},
	})

	// Only non-grpc servers: nothing is returned, however well addressed.
	register(&registry.Instance{
		ID:               uuid.New(),
		AdvertiseAddress: "10.0.0.4",
		Servers: map[string]*registry.Server{
			"tcp": {
				Type:             "tcp",
				AdvertiseAddress: "10.0.0.4",
				Port:             7000,
			},
			"udp": {
				Type:             "udp",
				AdvertiseAddress: "10.0.0.4",
				Port:             7001,
			},
		},
	})

	// Multiple instances: every endpoint must survive.
	for i := range 2 {
		register(&registry.Instance{
			ID:               uuid.New(),
			AdvertiseAddress: "10.0.2." + []string{"1", "2"}[i],
			Servers: map[string]*registry.Server{
				"grpc": {
					Type: "grpc",
					Port: 9201 + i,
				},
			},
		})
	}

	// There is deliberately no nil *Server case: registry.cloneInstance drops
	// nil server values before resolveGRPCAddrs sees them, so the srv == nil
	// guard cannot be reached through the public pool API.
	addrs := resolveGRPCAddrs(service)
	got := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		got = append(got, addr.Addr)
	}

	slices.Sort(got)

	want := []string{
		"10.0.0.10:9000",
		"10.0.0.2:9001",
		"10.0.2.1:9201",
		"10.0.2.2:9202",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("resolveGRPCAddrs = %v, want %v", got, want)
	}

	if unknown := resolveGRPCAddrs("no-such-service"); len(unknown) != 0 {
		t.Fatalf("unknown service returned %v, want no addresses", unknown)
	}
}

// TestWatchRegistryPoolUpdatesResolver proves the Service-mode watcher
// re-reads the pool and pushes the current addresses into the manual
// resolver, then stops when the client is disconnected.
func TestWatchRegistryPoolUpdatesResolver(t *testing.T) {
	pool := registry.InitPool()
	t.Cleanup(func() {
		registry.InitPool()
	})

	drainPoolNotify(pool)

	const service = "watch-test-svc"

	registry.RegisterService(&registry.Service{Service: service})
	registry.RegisterInstance(&registry.Instance{
		ID:               uuid.New(),
		ServiceName:      service,
		AdvertiseAddress: "127.0.0.1",
		Servers: map[string]*registry.Server{
			"grpc": {
				Type: "grpc",
				Port: 9100,
			},
		},
	})

	r := manual.NewBuilderWithScheme("sicky-test")
	cc := newRecordingResolverCC()

	target := resolver.Target{URL: url.URL{Scheme: "sicky-test", Host: service}}
	if _, err := r.Build(target, cc, resolver.BuildOptions{}); err != nil {
		t.Fatalf("build manual resolver: %v", err)
	}

	clt := &GRPCClient{done: make(chan struct{})}
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		watchRegistryPool(clt, r, service)
	}()

	// A purge replaces the pool and emits the changed event the watcher
	// listens for. The new port proves the watcher re-read the pool rather
	// than replaying the state it was constructed with.
	registry.PurgePool([]*registry.Instance{{
		ID:               uuid.New(),
		ServiceName:      service,
		AdvertiseAddress: "127.0.0.1",
		Servers: map[string]*registry.Server{
			"grpc": {
				Type: "grpc",
				Port: 9101,
			},
		},
	}})

	waitForResolverState(t, cc, []string{"127.0.0.1:9101"})

	close(clt.done)

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("watchRegistryPool kept running after done was closed")
	}
}

// TestWatchRegistryPoolWithoutPoolExitsOnDone covers the branch where the
// client exists before the pool does (NotifyChan returns nil): the watcher
// must retry, not panic or spin, and must still honor done. SetPool(nil) is
// deprecated for channel-swapping reasons, but it is the only way to
// un-install the global pool; cleanup restores one for the rest of the suite.
func TestWatchRegistryPoolWithoutPoolExitsOnDone(t *testing.T) {
	registry.SetPool(nil)
	t.Cleanup(func() {
		registry.InitPool()
	})

	if registry.NotifyChan() != nil {
		t.Fatal("NotifyChan must be nil without a pool")
	}

	r := manual.NewBuilderWithScheme("sicky-test-nil")
	cc := newRecordingResolverCC()

	target := resolver.Target{URL: url.URL{Scheme: "sicky-test-nil", Host: "svc"}}
	if _, err := r.Build(target, cc, resolver.BuildOptions{}); err != nil {
		t.Fatalf("build manual resolver: %v", err)
	}

	clt := &GRPCClient{done: make(chan struct{})}
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		watchRegistryPool(clt, r, "svc")
	}()

	close(clt.done)

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("watchRegistryPool kept running without a pool after done was closed")
	}
}

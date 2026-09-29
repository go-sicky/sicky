package broker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMessageJSONRoundTrip(t *testing.T) {
	type order struct {
		ID  string `json:"id"`
		Qty int    `json:"qty"`
	}

	m := &Message{Topic: "orders.created"}
	if err := m.Format(order{ID: "o1", Qty: 3}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if m.Mime != MsgJSON {
		t.Fatalf("Mime = %d, want MsgJSON(%d)", m.Mime, MsgJSON)
	}

	var got order
	if err := m.Scan(&got); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if got.ID != "o1" || got.Qty != 3 {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestMessageMsgpackRoundTrip(t *testing.T) {
	type item struct {
		Name string
		N    int
	}

	m := &Message{}
	if err := m.Format(item{Name: "x", N: 7}, MsgMessagePack); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if m.Mime != MsgMessagePack {
		t.Fatalf("Mime = %d, want MsgMessagePack", m.Mime)
	}

	var got item
	if err := m.Scan(&got); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if got.Name != "x" || got.N != 7 {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestMessageRawPassthrough(t *testing.T) {
	m := &Message{}
	if err := m.Format([]byte("bytes"), MsgRaw); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if m.Mime != MsgRaw || string(m.Body) != "bytes" {
		t.Fatalf("raw = mime %d body %q", m.Mime, m.Body)
	}
}

func TestMessageWireRoundTrip(t *testing.T) {
	src := &Message{Topic: "t"}
	if err := src.Format(map[string]string{"k": "v"}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	dst := NewMessage(src.Raw())
	if dst.Mime != MsgJSON || dst.Topic != "t" {
		t.Fatalf("wire = mime %d topic %q", dst.Mime, dst.Topic)
	}

	var got map[string]string
	if err := dst.Scan(&got); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if got["k"] != "v" {
		t.Fatalf("wire round trip = %v", got)
	}
}

func TestMessageFormatError(t *testing.T) {
	m := &Message{}
	if err := m.Format(func() {}); err == nil {
		t.Fatal("unmarshalable value must return error")
	}
}

type fakeBroker struct{ id uuid.UUID }

func (f *fakeBroker) Context() context.Context { return context.Background() }
func (f *fakeBroker) Options() *Options        { return nil }
func (f *fakeBroker) String() string           { return "fake" }
func (f *fakeBroker) Name() string             { return "fake" }
func (f *fakeBroker) ID() uuid.UUID            { return f.id }
func (f *fakeBroker) Connect() error           { return nil }
func (f *fakeBroker) Disconnect() error        { return nil }
func (f *fakeBroker) Publish(string, *Message) error {
	return errors.New("not connected")
}

func (f *fakeBroker) Subscribe(string, Handler) error { return nil }
func (f *fakeBroker) Unsubscribe(string) error        { return nil }

func TestRegistryConcurrentAccess(t *testing.T) {
	Clear()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			b := &fakeBroker{id: uuid.New()}
			Set(b)
			_ = Get(b.id)
			_ = Default()
			_ = Brokers()
		})
	}

	wg.Wait()
}

// blockingBroker is a fakeBroker whose Publish and Subscribe park until
// released, so a test can hold a call open inside the helper.
type blockingBroker struct {
	fakeBroker
	pubIn      chan struct{}
	pubRelease chan struct{}
	subIn      chan struct{}
	subRelease chan struct{}
}

func (b *blockingBroker) Publish(string, *Message) error {
	close(b.pubIn)
	<-b.pubRelease

	return nil
}

func (b *blockingBroker) Subscribe(string, Handler) error {
	close(b.subIn)
	<-b.subRelease

	return nil
}

func newBlockingBroker() *blockingBroker {
	return &blockingBroker{
		fakeBroker: fakeBroker{id: uuid.New()},
		pubIn:      make(chan struct{}),
		pubRelease: make(chan struct{}),
		subIn:      make(chan struct{}),
		subRelease: make(chan struct{}),
	}
}

// TestPublishHelperDoesNotHoldGlobalLockDuringNetworkCall guards the
// snapshot-then-release shape of the package-level helpers. Publish used to
// hold brkMu.RLock across defaultBroker.Publish, which for JetStream is a
// synchronous ack round-trip. A pending writer (Set, Clear) then parked and,
// because sync.RWMutex blocks later readers behind a pending writer, every
// subsequent Publish stalled for the full RTT.
//
// Negative control: with the old implementation this test times out, because
// Clear blocks until pubRelease is closed.
func TestPublishHelperDoesNotHoldGlobalLockDuringNetworkCall(t *testing.T) {
	Clear()

	blocking := newBlockingBroker()
	Set(blocking)

	published := make(chan error, 1)
	go func() { published <- Publish("topic", &Message{Body: []byte("x")}) }()

	<-blocking.pubIn // Publish is now parked inside the broker call.

	// A writer must be able to acquire brkMu while that call is in flight.
	acquired := make(chan struct{})
	go func() {
		Clear()
		close(acquired)
	}()

	select {
	case <-acquired:
		// Expected: the helper released brkMu before calling out.
	case <-time.After(2 * time.Second):
		close(blocking.pubRelease)
		t.Fatal("Clear blocked 2s while Publish was in flight: broker.Publish holds brkMu across the broker call")
	}

	close(blocking.pubRelease)

	if err := <-published; err != nil {
		t.Fatalf("publish: %v", err)
	}

	Clear()
}

// TestSubscribeHelperDoesNotHoldGlobalLockDuringNetworkCall is the Subscribe
// twin of the test above, for the same reason: a JetStream subscribe is a
// $JS.API.CONSUMER.* round-trip and must not run under brkMu.
func TestSubscribeHelperDoesNotHoldGlobalLockDuringNetworkCall(t *testing.T) {
	Clear()

	blocking := newBlockingBroker()
	Set(blocking)

	subscribed := make(chan error, 1)
	go func() { subscribed <- Subscribe("topic", nil) }()

	<-blocking.subIn // Subscribe is now parked inside the broker call.

	acquired := make(chan struct{})
	go func() {
		Clear()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		close(blocking.subRelease)
		t.Fatal("Clear blocked 2s while Subscribe was in flight: broker.Subscribe holds brkMu across the broker call")
	}

	close(blocking.subRelease)

	if err := <-subscribed; err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	Clear()
}

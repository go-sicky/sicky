package broker

import (
	"context"
	"errors"
	"sync"
	"testing"

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
	if m.Mime != MsgJson {
		t.Fatalf("Mime = %d, want MsgJson(%d)", m.Mime, MsgJson)
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
	if err := m.Format(item{Name: "x", N: 7}, MsgMsgpack); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if m.Mime != MsgMsgpack {
		t.Fatalf("Mime = %d, want MsgMsgpack", m.Mime)
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
	if dst.Mime != MsgJson || dst.Topic != "t" {
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
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b := &fakeBroker{id: uuid.New()}
			Set(b)
			_ = Get(b.id)
			_ = Default()
			_ = Brokers()
		}()
	}
	wg.Wait()
}

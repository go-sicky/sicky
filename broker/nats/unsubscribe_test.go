package nats

import (
	"maps"
	"testing"

	"github.com/go-sicky/sicky/broker"
)

// TestUnsubscribeForgetsReplayState guards the desired-state bookkeeping.
// brk.handlers is the list Connect replays on every reconnect, so an
// Unsubscribe that only removed the live subscription left the topic in
// the replay list: it resubscribed on the next reconnect and the handler
// closure was retained for the process lifetime.
func TestUnsubscribeForgetsReplayState(t *testing.T) {
	brk := New(&broker.Options{Name: "nats-unsubscribe-test"}, &Config{})
	if brk == nil {
		t.Fatal("New returned nil (config rejected)")
	}

	const topic = "orders.created"

	// Seed exactly the state Handle + a live Subscribe would leave.
	brk.mu.Lock()
	brk.handlers[topic] = func(*broker.Message) error { return nil }
	brk.mu.Unlock()

	// Unsubscribe with no live subscription must still forget the
	// replay entry: that is the case the audit flagged, where the topic
	// was registered for replay but never (or no longer) subscribed.
	if err := brk.Unsubscribe(topic); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	brk.mu.RLock()
	_, inHandlers := brk.handlers[topic]
	_, inSubs := brk.subscriptions[topic]
	replay := maps.Clone(brk.handlers)
	brk.mu.RUnlock()

	if inHandlers {
		t.Fatal("topic is still in brk.handlers: Connect would resubscribe it on the next reconnect")
	}

	if inSubs {
		t.Fatal("topic is still in brk.subscriptions after Unsubscribe")
	}

	// Replay simulation: this is the exact snapshot+iterate Connect does.
	for replayed := range replay {
		if replayed == topic {
			t.Fatal("the Connect replay set still contains the unsubscribed topic")
		}
	}

	// A sibling topic must survive: Unsubscribe forgets one topic, not
	// the whole replay list.
	const other = "orders.cancelled"

	brk.mu.Lock()
	brk.handlers[other] = func(*broker.Message) error { return nil }
	brk.mu.Unlock()

	if err := brk.Unsubscribe(topic); err != nil {
		t.Fatalf("Unsubscribe (second): %v", err)
	}

	brk.mu.RLock()
	_, otherKept := brk.handlers[other]
	brk.mu.RUnlock()

	if !otherKept {
		t.Fatal("Unsubscribe dropped an unrelated replay entry")
	}
}

package broker

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The data-plane helpers must not report success when no broker is
// registered: a nil return tells the caller a message was transmitted
// when it was dropped on the floor.
func TestDataPlaneHelpersReportUninitialized(t *testing.T) {
	Clear()

	if err := Publish("orders", &Message{}); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Publish without a broker: %v, want %v", err, ErrNotInitialized)
	}

	if err := Subscribe("orders", func(*Message) error { return nil }); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Subscribe without a broker: %v, want %v", err, ErrNotInitialized)
	}
}

// Unsubscribe has no work to drop, so nil stays correct: the caller's
// intent (no subscription for this topic) already holds.
func TestUnsubscribeStaysNilSafe(t *testing.T) {
	Clear()

	if err := Unsubscribe("orders"); err != nil {
		t.Fatalf("Unsubscribe without a broker must be nil, got %v", err)
	}
}

// Once a broker is registered the helpers must delegate instead of
// reporting the sentinel, or the fix would break every real deployment.
func TestDataPlaneHelpersDelegateWhenInitialized(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	Set(&fakeBroker{id: uuid.New()})
	if Default() == nil {
		t.Fatal("Default must be set after Set")
	}

	// fakeBroker.Publish returns its own "not connected" error, so
	// seeing that exact message proves the helper forwarded the call
	// rather than answering from its own "no broker registered" path.
	err := Publish("orders", &Message{})
	if err == nil {
		t.Fatal("Publish must return the registered broker's own error")
	}

	if errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Publish with a broker registered returned the uninitialized sentinel: %v", err)
	}

	if err.Error() != "not connected" {
		t.Fatalf("Publish returned %q, want the fake broker's own error", err.Error())
	}

	if err := Subscribe("orders", func(*Message) error { return nil }); err != nil {
		t.Fatalf("Subscribe with a broker registered: %v, want nil", err)
	}
}

// The sentinel must survive a Clear/Set cycle, i.e. it is a package-level
// value rather than something rebuilt per call.
func TestSentinelIsStableAcrossClear(t *testing.T) {
	Clear()

	first := ErrNotInitialized
	Set(&fakeBroker{id: uuid.New()})
	Clear()

	// errors.Is rather than ==: it still proves the value is unchanged
	// and keeps working if the sentinel ever becomes a wrapped error.
	if !errors.Is(ErrNotInitialized, first) {
		t.Fatal("the sentinel must be a stable package-level value")
	}

	if err := Publish("orders", &Message{}); !errors.Is(err, first) {
		t.Fatalf("after a Clear/Set cycle Publish returned %v, want %v", err, first)
	}
}

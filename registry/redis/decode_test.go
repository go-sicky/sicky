/**
 * @file decode_test.go
 * @package redis
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package redis

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/registry"
)

func instanceJSON(t *testing.T, ins *registry.Instance) string {
	t.Helper()

	data, err := json.Marshal(ins)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return string(data)
}

// One unreadable value must not take the whole load down, and must not take
// the metrics with it.
func TestDecodeInstancesSkipsBadValuesAndKeepsTheRest(t *testing.T) {
	good := &registry.Instance{
		ID:             uuid.New(),
		ServiceName:    "svc",
		ManagerAddress: "127.0.0.1",
		ManagerPort:    9999,
	}

	raw := map[string]string{
		"a-good": instanceJSON(t, good),
		"b-bad":  "{not json",
		"c-good": instanceJSON(t, &registry.Instance{ID: uuid.New(), ServiceName: "other"}),
	}

	var failed []string

	got := decodeInstances(raw, func(field string, err error) {
		failed = append(failed, field)
	})

	if len(got) != 2 {
		t.Fatalf("decoded %d instances, want 2: one bad value must not discard the rest", len(got))
	}

	for _, ins := range got {
		if ins.ServiceName == "" || ins.ID == uuid.Nil {
			t.Errorf("decoded instance is empty: %+v", ins)
		}
	}

	if len(failed) != 1 || failed[0] != "b-bad" {
		t.Errorf("failed = %v, want exactly the offending field named", failed)
	}
}

// Every value unreadable, and every one reported: the decode must not stop at
// the first failure, because the rest of the hash is still good data.
func TestDecodeInstancesReportsEveryFailure(t *testing.T) {
	raw := map[string]string{"a": "x", "b": "y"}

	called := 0

	got := decodeInstances(raw, func(string, error) { called++ })

	if len(got) != 0 {
		t.Fatalf("decoded %d instances from garbage, want 0", len(got))
	}

	if called != 2 {
		t.Errorf("onError called %d times, want 2", called)
	}
}

// The signature itself is the guarantee: the decode reports through its own
// callback and returns nothing, so there is no path by which a per-entry
// failure can reach Load's named error and be recorded as an operation
// failure. The compile-time assertion in decode.go pins this from the other
// side; this test pins that the callback is still used.
func TestDecodeInstancesUsesTheCallbackForFailuresOnly(t *testing.T) {
	good := instanceJSON(t, &registry.Instance{ID: uuid.New(), ServiceName: "ok"})

	raw := map[string]string{"good": good, "bad": "{"}

	called := 0

	got := decodeInstances(raw, func(string, error) { called++ })

	if len(got) != 1 || called != 1 {
		t.Errorf("decoded %d, failures %d; want 1 and 1", len(got), called)
	}
}

// A nil callback must not panic: the decoder is called from Load with a
// logger, but a caller without one is not a programming error worth a crash.
func TestDecodeInstancesToleratesNilCallback(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil onError panicked: %v", r)
		}
	}()

	got := decodeInstances(map[string]string{"a": "not json"}, nil)
	if len(got) != 0 {
		t.Errorf("decoded %d instances from garbage, want 0", len(got))
	}
}

// An empty hash is a legitimate state — no service registered yet — and must
// not be reported as a failure.
func TestDecodeInstancesHandlesEmptyHash(t *testing.T) {
	got := decodeInstances(nil, func(string, error) {
		t.Error("onError fired for an empty hash")
	})

	if len(got) != 0 {
		t.Errorf("decoded %d instances from nothing, want 0", len(got))
	}
}

// The error the callback receives must be the decode failure, not something
// the decoder invented.
func TestDecodeInstancesReportsTheUnderlyingError(t *testing.T) {
	var got error

	decodeInstances(map[string]string{"a": "{not json"}, func(_ string, err error) {
		got = err
	})

	if got == nil {
		t.Fatal("no error reported for an unparseable value")
	}

	if _, ok := errors.AsType[*json.SyntaxError](got); !ok {
		t.Errorf("error = %v, want the json syntax error so the log names the cause", got)
	}
}

/**
 * @file message_format_test.go
 * @package broker
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package broker

import (
	"bytes"
	"errors"
	"testing"
)

// json.Marshal and msgpack.Marshal both assign their result to m.Body before
// the error is examined. On failure that result is nil, so a marshal error
// silently wiped whatever payload the message already carried and left Mime
// claiming a codec that no longer describes the body.
//
// Raw() already guards the same hazard for its own path — it falls back to
// m.Body rather than sending an empty payload. Format had no equivalent, so
// the half-written message was one unchecked error away from being published:
// Mime says JSON, Body is empty, and Scan on the far side decodes nothing and
// reports no error.
func TestFormatFailureLeavesTheMessageUntouched(t *testing.T) {
	for _, tc := range []struct {
		name string
		mime int
	}{
		{name: "json", mime: MsgJSON},
		{name: "msgpack", mime: MsgMessagePack},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(`{"id":"already-here"}`)

			m := &Message{Topic: "t", Mime: MsgRaw, Body: bytes.Clone(original)}

			// A func is unmarshalable by both codecs.
			if err := m.Format(func() {}, tc.mime); err == nil {
				t.Fatal("an unmarshalable value must return an error")
			}

			if !bytes.Equal(m.Body, original) {
				t.Errorf("body = %q, want the original %q: a failed encode must not "+
					"wipe a payload the message already carried", m.Body, original)
			}

			if m.Mime != MsgRaw {
				t.Errorf("mime = %d, want %d (Raw): the codec must only change on a "+
					"successful encode", m.Mime, MsgRaw)
			}
		})
	}
}

// The end-to-end harm: before the fix a failed Format left Mime claiming a
// codec over an empty body, so the message went out empty and the receiver's
// Scan decoded nothing while reporting success. A message that still carries
// its payload through the wire after a failed encode is the observable form of
// the fix.
func TestFormatFailureStillDeliversTheOriginalPayload(t *testing.T) {
	m := &Message{Topic: "t", Mime: MsgRaw, Body: []byte(`{"id":"keep-me"}`)}

	if err := m.Format(func() {}, MsgJSON); err == nil {
		t.Fatal("expected an error")
	}

	// Over the wire and back, the way a broker would carry it.
	got := NewMessage(m.Raw())

	if !bytes.Equal(got.Body, []byte(`{"id":"keep-me"}`)) {
		t.Errorf("delivered body = %q, want the original payload; a failed encode "+
			"must not turn a carried message into an empty one", got.Body)
	}
}

// The successful path is unaffected: a successful encode replaces the body and
// records the codec.
func TestFormatSuccessStillReplacesBodyAndMime(t *testing.T) {
	m := &Message{Topic: "t", Mime: MsgRaw, Body: []byte("stale")}

	if err := m.Format(map[string]string{"k": "v"}, MsgJSON); err != nil {
		t.Fatalf("format: %v", err)
	}

	if m.Mime != MsgJSON {
		t.Errorf("mime = %d, want %d", m.Mime, MsgJSON)
	}

	if bytes.Contains(m.Body, []byte("stale")) {
		t.Errorf("body = %q, want the new payload: a successful encode replaces it", m.Body)
	}
}

// A raw-format failure must not change the mime either: the Raw branch only
// sets MsgRaw when the value really is bytes.
func TestRawFormatFailureLeavesMimeAlone(t *testing.T) {
	m := &Message{Topic: "t", Mime: MsgJSON, Body: []byte("keep")}

	err := m.Format("not bytes", MsgRaw)
	if err == nil {
		t.Fatal("a non-[]byte value with the raw mime must return an error")
	}

	if !errors.Is(err, ErrFormatNotBytes) {
		t.Errorf("error = %v, want ErrFormatNotBytes so the caller can tell why", err)
	}

	if m.Mime != MsgJSON {
		t.Errorf("mime = %d, want it unchanged at %d", m.Mime, MsgJSON)
	}

	if !bytes.Equal(m.Body, []byte("keep")) {
		t.Errorf("body = %q, want it unchanged", m.Body)
	}
}

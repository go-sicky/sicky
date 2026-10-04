package logger

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// decodeJSON is json.Unmarshal under a name the test file owns, so the
// helper above reads without importing an alias at every call.
func decodeJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

// recording builds a GRPC logger whose records land in buf.
func recording(buf *strings.Builder) GRPCLogger {
	ins := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: lTrace}))

	return NewGRPC(ins)
}

// extractMsg pulls the "msg" field out of a single JSON record. It decodes
// rather than string-matching so an escaped newline is inspected as the
// character it is, which is the whole point: the defect is invisible unless
// you decode.
func extractMsg(t *testing.T, line string) string {
	t.Helper()

	var rec struct {
		Msg string `json:"msg"`
	}

	if err := decodeJSON([]byte(line), &rec); err != nil {
		t.Fatalf("record %q is not valid JSON: %v", line, err)
	}

	return rec.Msg
}

// The ln family exists to reproduce the stdlib log spacing — operands
// separated by a space. It also inherited stdlib's trailing newline, but
// here that newline lands inside the message rather than terminating a line:
// the record already ends with one, so every ln call emitted
// {"msg":"hello world\n"} followed by its own record terminator.
//
// A log consumer that splits records on newlines sees one record; a
// consumer that reads msg sees a value with a stray control character at
// the end, and every downstream comparison against the intended text fails.
func TestLnFamilyDoesNotEmbedATrailingNewline(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(GRPCLogger)
		want string
	}{
		{name: "Infoln", call: func(l GRPCLogger) { l.Infoln("hello", "world") }, want: "hello world"},
		{name: "Warningln", call: func(l GRPCLogger) { l.Warningln("warn me") }, want: "warn me"},
		{name: "Errorln", call: func(l GRPCLogger) { l.Errorln("failed") }, want: "failed"},
		{name: "Fatalln is unreachable", call: nil},
	} {
		if tc.call == nil {
			// Fatalln ends the process, so it cannot be asserted here. The
			// same helper produces its message, so covering the three
			// reachable methods covers the shared cause.
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			buf := &strings.Builder{}
			tc.call(recording(buf))

			raw := buf.String()
			if !strings.HasSuffix(raw, "}\n") {
				t.Fatalf("record = %q, want it to end with the handler's own newline", raw)
			}

			if got := extractMsg(t, strings.TrimSuffix(raw, "\n")); got != tc.want {
				t.Errorf("msg = %q, want %q: the handler already terminates the "+
					"record, so a newline inside msg is a stray character", got, tc.want)
			}
		})
	}
}

// One operand: the newline is the only thing being removed, so this pins
// that the fix is a trim rather than a change of spacing.
func TestLnFamilySingleOperandIsUnchangedApartFromTheNewline(t *testing.T) {
	buf := &strings.Builder{}
	recording(buf).Errorln("only")

	if got := extractMsg(t, strings.TrimSuffix(buf.String(), "\n")); got != "only" {
		t.Errorf("msg = %q, want %q", got, "only")
	}
}

// No operands at all: Sprintln yields just the newline, so the message
// becomes empty rather than a bare control character.
func TestLnFamilyWithNoOperandsProducesAnEmptyMessage(t *testing.T) {
	buf := &strings.Builder{}
	recording(buf).Infoln()

	if got := extractMsg(t, strings.TrimSuffix(buf.String(), "\n")); got != "" {
		t.Errorf("msg = %q, want empty", got)
	}
}

// The non-ln family must keep fmt.Sprint semantics (no separator between
// adjacent strings), which is what stdlib log.Print does. The fix must not
// have leaked a space in there.
func TestPrintFamilyKeepsSprintSpacing(t *testing.T) {
	buf := &strings.Builder{}
	recording(buf).Info("ab", "cd")

	if got := extractMsg(t, strings.TrimSuffix(buf.String(), "\n")); got != "abcd" {
		t.Errorf("msg = %q, want %q: fmt.Sprint inserts no separator between "+
			"adjacent strings, matching stdlib log.Print", got, "abcd")
	}
}

// Info/Infof must never gain a trailing newline either; this guards the
// sibling methods against a future copy of the ln fix.
func TestPrintAndFormatFamilyCarryNoTrailingNewline(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(GRPCLogger)
	}{
		{name: "Info", call: func(l GRPCLogger) { l.Info("x") }},
		{name: "Infof", call: func(l GRPCLogger) { l.Infof("%s", "x") }},
		{name: "Warning", call: func(l GRPCLogger) { l.Warning("x") }},
		{name: "Error", call: func(l GRPCLogger) { l.Error("x") }},
		{name: "Errorf", call: func(l GRPCLogger) { l.Errorf("%s", "x") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := &strings.Builder{}
			tc.call(recording(buf))

			if got := extractMsg(t, strings.TrimSuffix(buf.String(), "\n")); got != "x" {
				t.Errorf("msg = %q, want %q", got, "x")
			}
		})
	}
}

// Fatalln ends the process, so it cannot be exercised through the logger.
// spaced is the single cause behind all four ln methods, so asserting it
// directly is what actually covers Fatalln rather than leaving it out.
func TestSpacedDropsExactlyTheOneNewlineSprintlnAdds(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []any
		want string
	}{
		{name: "none", args: nil, want: ""},
		{name: "one string", args: []any{"only"}, want: "only"},
		{name: "two strings are space separated", args: []any{"a", "b"}, want: "a b"},
		{name: "string and int", args: []any{"n", 7}, want: "n 7"},
		// The caller's own trailing newline must survive: only the one
		// Sprintln appends is trimmed, and TrimSuffix removes one.
		{name: "caller newline kept", args: []any{"a\n"}, want: "a\n"},
		{name: "blank line kept", args: []any{"a\n\n"}, want: "a\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := spaced(tc.args...); got != tc.want {
				t.Errorf("spaced(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

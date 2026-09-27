package protocol

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestStdioReadRejectsOversizedMessage: ReadBytes used to grow its
// buffer without limit, so a hostile or garbled stream could exhaust
// memory line by line.
func TestStdioReadRejectsOversizedMessage(t *testing.T) {
	long := strings.Repeat("a", maxMessageBytes+1024) + "\n"
	tr := &StdioTransport{
		reader: bufio.NewReaderSize(strings.NewReader(long), maxMessageBytes),
		writer: io.Discard,
	}

	if _, err := tr.Read(); err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("oversized line error = %v, want the size-limit error", err)
	}
}

// TestStdioReadClonesLine: ReadSlice hands back a view into the buffer;
// the caller must not retain it.
func TestStdioReadClonesLine(t *testing.T) {
	tr := &StdioTransport{
		reader: bufio.NewReaderSize(strings.NewReader("first\nsecond\n"), 4096),
		writer: io.Discard,
	}

	first, err := tr.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if _, err := tr.Read(); err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(first) != "first" {
		t.Fatalf("first line = %q, want it unaffected by the second read", first)
	}
}

package interactive

import (
	"bufio"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-sicky/sicky/service"
)

// newTestService wires an in-memory stdin over the reader the loop uses.
func newTestService(t *testing.T, stdin io.Reader) *Interactive {
	t.Helper()

	svc := New(&service.Options{Name: "interactive-test"}, &Config{})
	svc.stdin = stdin
	svc.reader = bufio.NewReader(svc.stdin)

	return svc
}

// recordingHandler captures the commands the loop dispatched.
type recordingHandler struct {
	name  string
	cmds  []string
	stops int
}

func (h *recordingHandler) Name() string { return h.name }

func (h *recordingHandler) OnInteract(cmd, full string) error {
	h.cmds = append(h.cmds, cmd)

	return nil
}

func (h *recordingHandler) OnStop() error {
	h.stops++

	return nil
}

// TestLoopEndsOnEOF is the regression guard for the CPU-spinning bug:
// any stdin read error (io.EOF included) used to return "keep going",
// so a closed stdin (`sicky serve < /dev/null`) re-prompted forever.
func TestLoopEndsOnEOF(t *testing.T) {
	svc := newTestService(t, strings.NewReader(""))

	returned := make(chan struct{})
	go func() {
		svc.loop()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not stop on EOF: stdin read error was treated as retry")
	}
}

func TestInteractReturnsDoneOnEOF(t *testing.T) {
	svc := newTestService(t, strings.NewReader(""))

	if got := svc.interact(); got != interactDone {
		t.Fatalf("empty stdin: interact() = %v, want interactDone", got)
	}
}

func TestInteractDispatchesFinalLineWithoutNewline(t *testing.T) {
	h := &recordingHandler{name: "rec"}
	svc := newTestService(t, strings.NewReader("status"))
	svc.Handle(h)

	if got := svc.interact(); got != interactDone {
		t.Fatalf("interact() = %v, want interactDone after EOF", got)
	}
	if len(h.cmds) != 1 || h.cmds[0] != "status" {
		t.Fatalf("dispatched commands = %v, want [status]", h.cmds)
	}
}

// TestInteractKeepsBufferedLines guards the per-turn reader rebuild:
// buffering stdin once keeps every piped line instead of dropping all
// but the first.
func TestInteractKeepsBufferedLines(t *testing.T) {
	h := &recordingHandler{name: "rec"}
	svc := newTestService(t, strings.NewReader("one\ntwo\nthree\n"))
	svc.Handle(h)

	for i := range 3 {
		if got := svc.interact(); got != interactContinue {
			t.Fatalf("turn %d: interact() = %v, want interactContinue", i, got)
		}
	}

	if got := svc.interact(); got != interactDone {
		t.Fatalf("after input: interact() = %v, want interactDone", got)
	}
	if len(h.cmds) != 3 {
		t.Fatalf("dispatched commands = %v, want 3 entries", h.cmds)
	}
}

func TestInteractQuitOnStopCommand(t *testing.T) {
	h := &recordingHandler{name: "rec"}
	svc := newTestService(t, strings.NewReader(DefaultStopCommand+"\n"))
	svc.Handle(h)

	if got := svc.interact(); got != interactQuit {
		t.Fatalf("interact() = %v, want interactQuit", got)
	}
	if h.stops != 1 {
		t.Fatalf("OnStop calls = %d, want 1", h.stops)
	}
}

type countingReader struct {
	reader *strings.Reader
	reads  int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++

	return r.reader.Read(p)
}

// TestStopStopsFurtherReads: after Stop the loop must not prompt or read
// stdin again (a read already in flight cannot be canceled).
func TestStopStopsFurtherReads(t *testing.T) {
	src := &countingReader{reader: strings.NewReader("one\ntwo\n")}
	svc := newTestService(t, src)

	if errs := svc.Stop(); len(errs) != 0 {
		t.Fatalf("stop errors: %v", errs)
	}

	// Stop is idempotent: a second call must not panic on close.
	if errs := svc.Stop(); len(errs) != 0 {
		t.Fatalf("second stop errors: %v", errs)
	}

	if got := svc.interact(); got != interactDone {
		t.Fatalf("interact() after Stop = %v, want interactDone", got)
	}
	if src.reads != 0 {
		t.Fatalf("stdin was read %d times after Stop", src.reads)
	}
}

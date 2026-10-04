package ticker

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/metrics"
)

func runs(taskID, result string) float64 {
	return testutil.ToFloat64(metrics.JobRunsTotal.WithLabelValues("ticker", taskID, result))
}

// The ticker loop classifies the run outcome, and it used to do so by reading
// the error text: strings.Contains on "timed out" / "panicked". A handler
// whose own failure mentions a timeout was therefore counted as a watchdog
// timeout — indistinguishable, in the metric an operator reads for exactly
// that question, from this task genuinely exceeding its budget.
func TestLoopCountsAMisleadingHandlerErrorAsError(t *testing.T) {
	taskID := uuid.New()
	entered := make(chan struct{}, 8)

	j := New(&job.Options{ID: uuid.New(), Name: "metrics"}, &Config{Interval: 1})

	if err := j.Add(&Task{
		ID:      taskID,
		Inteval: 1,
		Handler: func(time.Time, uint64) error {
			select {
			case entered <- struct{}{}:
			default:
			}

			return errors.New("upstream request timed out after 30s")
		},
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}

	beforeErr := runs(taskID.String(), job.ResultError)
	beforeTimeout := runs(taskID.String(), job.ResultTimeout)

	if err := j.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() {
		if err := j.Stop(); err != nil {
			t.Errorf("stop: %v", err)
		}
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never ran")
	}

	// The metric is observed in a deferred call in the same statement block as
	// the handler, so give it a moment to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runs(taskID.String(), job.ResultError) > beforeErr {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if got := runs(taskID.String(), job.ResultError) - beforeErr; got < 1 {
		t.Errorf("result=error moved by %v, want at least +1", got)
	}

	if got := runs(taskID.String(), job.ResultTimeout) - beforeTimeout; got != 0 {
		t.Errorf("result=timeout moved by %v, want 0: no watchdog fired. The handler "+
			"failed on its own terms and only its wording mentioned a timeout", got)
	}
}

// A real watchdog expiry must still be counted, or the classification has
// simply stopped counting the case the label exists for.
func TestWatchdogExpiryReturnsTheTimeoutSentinel(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "sentinel"}, &Config{Interval: 1})

	err := j.runWithTimeout(&Task{
		ID:      uuid.New(),
		Timeout: 20 * time.Millisecond,
		Handler: func(time.Time, uint64) error {
			time.Sleep(300 * time.Millisecond)

			return nil
		},
	}, time.Now(), 1)
	if !errors.Is(err, job.ErrTaskTimeout) {
		t.Fatalf("error = %v, want one wrapping job.ErrTaskTimeout", err)
	}
}

// A panicking handler must be recoverable and identifiable, so the caller can
// distinguish a crashing handler from a failing one without reading text.
func TestPanickingHandlerReturnsThePanicSentinel(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "sentinel"}, &Config{Interval: 1})

	err := j.runWithTimeout(&Task{
		ID:      uuid.New(),
		Timeout: time.Second,
		Handler: func(time.Time, uint64) error {
			panic("boom")
		},
	}, time.Now(), 1)
	if !errors.Is(err, job.ErrTaskPanic) {
		t.Fatalf("error = %v, want one wrapping job.ErrTaskPanic", err)
	}
}

// A handler error that merely mentions a panic must not be reported as one.
func TestHandlerErrorMentioningAPanicIsNotThePanicSentinel(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "sentinel"}, &Config{Interval: 1})

	err := j.runWithTimeout(&Task{
		ID:      uuid.New(),
		Timeout: time.Second,
		Handler: func(time.Time, uint64) error {
			return errors.New("remote handler panicked, retrying")
		},
	}, time.Now(), 1)
	if err == nil {
		t.Fatal("expected the handler error to propagate")
	}

	if errors.Is(err, job.ErrTaskPanic) {
		t.Errorf("error = %v: a handler error mentioning a panic must not carry the "+
			"panic sentinel", err)
	}
}

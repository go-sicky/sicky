package cron

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/metrics"
)

// runs reads the sicky_job_runs_total counter for one (job, task, result).
func runs(kind, taskID, result string) float64 {
	return testutil.ToFloat64(metrics.JobRunsTotal.WithLabelValues(kind, taskID, result))
}

// A handler whose own failure happens to mention a timeout must be counted as
// an error, not as a watchdog timeout. Before identity-based classification
// this was decided by strings.Contains on the error text, so a downstream
// service being slow was indistinguishable from this task exceeding its own
// budget — in the exact metric an operator reads to tell those apart.
func TestHandlerErrorMentioningATimeoutIsCountedAsError(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "metrics"}, &Config{})
	task := &Task{ID: uuid.New()}

	before := map[string]float64{
		"error":   runs("cron", task.ID.String(), job.ResultError),
		"timeout": runs("cron", task.ID.String(), job.ResultTimeout),
	}

	wrapped := j.runWithTimeout(task, func() error {
		return errors.New("upstream request timed out after 30s")
	})

	if err := wrapped(); err == nil {
		t.Fatal("handler error must propagate")
	}

	if got := runs("cron", task.ID.String(), job.ResultError) - before["error"]; got != 1 {
		t.Errorf("result=error moved by %v, want +1: the handler failed on its own terms", got)
	}

	if got := runs("cron", task.ID.String(), job.ResultTimeout) - before["timeout"]; got != 0 {
		t.Errorf("result=timeout moved by %v, want 0: no watchdog fired, so this is "+
			"not a timeout", got)
	}
}

// The genuine article must still land on timeout, or the fix has simply
// stopped counting the case the label exists for.
func TestWatchdogExpiryIsStillCountedAsTimeout(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "metrics"}, &Config{})
	task := &Task{ID: uuid.New(), Timeout: 20 * time.Millisecond}

	before := runs("cron", task.ID.String(), job.ResultTimeout)

	wrapped := j.runWithTimeout(task, func() error {
		time.Sleep(300 * time.Millisecond)

		return nil
	})

	err := wrapped()
	if !errors.Is(err, job.ErrTaskTimeout) {
		t.Fatalf("error = %v, want one wrapping job.ErrTaskTimeout so callers can "+
			"distinguish a watchdog verdict without reading text", err)
	}

	if got := runs("cron", task.ID.String(), job.ResultTimeout) - before; got != 1 {
		t.Errorf("result=timeout moved by %v, want +1", got)
	}
}

// A recovered panic is a distinct outcome from an ordinary failure, and the
// label is how an operator tells a crashing handler from a failing one.
func TestPanickingHandlerIsCountedAsPanic(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "metrics"}, &Config{})
	task := &Task{ID: uuid.New(), Timeout: time.Second}

	before := map[string]float64{
		"panic": runs("cron", task.ID.String(), job.ResultPanic),
		"error": runs("cron", task.ID.String(), job.ResultError),
	}

	wrapped := j.runWithTimeout(task, func() error {
		panic("boom")
	})

	err := wrapped()
	if !errors.Is(err, job.ErrTaskPanic) {
		t.Fatalf("error = %v, want one wrapping job.ErrTaskPanic", err)
	}

	if got := runs("cron", task.ID.String(), job.ResultPanic) - before["panic"]; got != 1 {
		t.Errorf("result=panic moved by %v, want +1", got)
	}

	if got := runs("cron", task.ID.String(), job.ResultError) - before["error"]; got != 0 {
		t.Errorf("result=error moved by %v, want 0: a panic is not an error result", got)
	}
}

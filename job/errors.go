/**
 * @file errors.go
 * @package job
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package job

import "errors"

// Sentinel errors for the task watchdog. The implementations wrap these so a
// caller — and the metrics label — can tell a watchdog verdict from an
// ordinary handler failure by identity.
//
// The distinction used to be made by reading the error text
// (strings.Contains on "timed out" / "panicked"), which classifies by
// wording rather than by what actually happened. "upstream request timed out
// after 30s" is an extremely ordinary error for a handler to return, and it
// was recorded as a watchdog timeout: the one metric an operator consults to
// decide whether their jobs are too slow was reporting handler failures as
// timeouts, and the reverse.
var (
	// ErrTaskTimeout is returned by the watchdog when a task exceeds its
	// configured Timeout. The run itself is not cancellable and keeps
	// going in the background.
	ErrTaskTimeout = errors.New("job: task timed out")
	// ErrTaskPanic is returned when a task handler panicked. The panic is
	// recovered so one bad run cannot take the scheduler with it.
	ErrTaskPanic = errors.New("job: task panicked")
)

// Result labels for sicky_job_runs_total. They are returned by Classify and
// match the label values the metric is documented to carry.
const (
	// ResultOK marks a task that returned no error.
	ResultOK = "ok"
	// ResultError marks a task that returned an error of its own.
	ResultError = "error"
	// ResultTimeout marks a run the watchdog gave up on.
	ResultTimeout = "timeout"
	// ResultPanic marks a run whose handler panicked.
	ResultPanic = "panic"
)

// Classify maps a task's error to a sicky_job_runs_total result label.
//
// It keys off error identity, not message text: ErrTaskTimeout and
// ErrTaskPanic are wrapped by the watchdog, and anything else is the
// handler's own failure no matter how it is worded. A nil error is ResultOK.
//
// Both job backends route their metric through this so the label means the
// same thing for cron and for ticker.
func Classify(err error) string {
	switch {
	case err == nil:
		return ResultOK
	case errors.Is(err, ErrTaskTimeout):
		return ResultTimeout
	case errors.Is(err, ErrTaskPanic):
		return ResultPanic
	default:
		return ResultError
	}
}

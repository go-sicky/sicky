/**
 * @file errors_test.go
 * @package job
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package job

import (
	"errors"
	"fmt"
	"testing"
)

// Classify used to be a pair of strings.Contains calls on the error text, so a
// watchdog verdict was distinguished from a handler failure by wording alone.
//
// "upstream request timed out after 30s" is an entirely ordinary error for a
// handler to return — the downstream service being slow is the *handler's*
// problem, and it is reported as one. But it was counted as a watchdog
// timeout, in the one metric an operator consults to decide whether their
// jobs are too slow. The mirror case is worse in principle: a genuine panic
// wrapped by text that happens not to contain the magic word would land in
// the generic bucket.
func TestClassifyDoesNotReadTheMessage(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		// The two cases the text sniff got wrong.
		{
			name: "handler error mentioning a timeout",
			err:  errors.New("upstream request timed out after 30s"),
			want: ResultError,
		},
		{
			name: "handler error mentioning a panic",
			err:  errors.New("remote handler panicked, retrying"),
			want: ResultError,
		},
		{
			name: "wrapped handler error mentioning both",
			err:  fmt.Errorf("sync: %w", errors.New("peer timed out and then panicked")),
			want: ResultError,
		},
		// Ordinary failures.
		{name: "nil", err: nil, want: ResultOK},
		{name: "plain error", err: errors.New("disk full"), want: ResultError},
		// Real watchdog verdicts, by identity rather than by text.
		{
			name: "timeout sentinel",
			err:  ErrTaskTimeout,
			want: ResultTimeout,
		},
		{
			name: "wrapped timeout sentinel",
			err:  fmt.Errorf("%w (task abc) after 5s", ErrTaskTimeout),
			want: ResultTimeout,
		},
		{
			name: "panic sentinel",
			err:  ErrTaskPanic,
			want: ResultPanic,
		},
		{
			name: "wrapped panic sentinel",
			err:  fmt.Errorf("%w (task abc): runtime error: index out of range", ErrTaskPanic),
			want: ResultPanic,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.err); got != tc.want {
				t.Errorf("Classify(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// A watchdog verdict that a caller wrapped for context must still be
// recognizable — errors.Is walks the chain, which is the whole reason the
// sentinels exist instead of the magic strings.
func TestClassifySeesThroughCallerWrapping(t *testing.T) {
	wrapped := fmt.Errorf("scheduling %s job: %w", "nightly", fmt.Errorf("%w after 1s", ErrTaskTimeout))

	if got := Classify(wrapped); got != ResultTimeout {
		t.Errorf("Classify = %q, want %q: identity must survive arbitrary wrapping", got, ResultTimeout)
	}

	if !errors.Is(wrapped, ErrTaskTimeout) {
		t.Error("errors.Is must find the sentinel through the wrapping")
	}
}

// The result labels are part of the metric's documented contract
// (ok/error/timeout/panic), so pin the exact strings rather than the
// constants — a renamed constant would otherwise pass unnoticed while the
// dashboard silently loses a series.
func TestResultLabelValuesAreTheDocumentedOnes(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{ResultOK, "ok"},
		{ResultError, "error"},
		{ResultTimeout, "timeout"},
		{ResultPanic, "panic"},
	} {
		if tc.got != tc.want {
			t.Errorf("label %q, want %q: a change here splits the metric series "+
				"and every existing dashboard", tc.got, tc.want)
		}
	}
}

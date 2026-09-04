package cron

import (
	"strings"
	"testing"
	"time"

	"github.com/go-sicky/sicky/job"
	"github.com/google/uuid"
)

func TestRunWithTimeoutExpiry(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	task := &Task{ID: uuid.New(), Timeout: 20 * time.Millisecond}
	wrapped := j.runWithTimeout(task, func() error {
		time.Sleep(500 * time.Millisecond)
		return nil
	})
	start := time.Now()
	err := wrapped()
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if time.Since(start) > 400*time.Millisecond {
		t.Fatal("watchdog did not fire promptly")
	}
}

func TestRunWithTimeoutPassthrough(t *testing.T) {
	j := New(&job.Options{ID: uuid.New(), Name: "test"}, &Config{})
	task := &Task{ID: uuid.New()}
	wrapped := j.runWithTimeout(task, func() error { return nil })
	if err := wrapped(); err != nil {
		t.Fatalf("zero timeout must pass through, got %v", err)
	}
}

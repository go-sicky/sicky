/**
 * @file panic_observability_test.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package grpc

import (
	"context"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// recordedEntry is one log call captured by recordingLogger.
type recordedEntry struct {
	level logger.Level
	msg   string
	args  []any
}

// field returns the value logged under key in a fixed-order args slice.
func (e recordedEntry) field(key string) any {
	for i := 0; i+1 < len(e.args); i += 2 {
		if s, ok := e.args[i].(string); ok && s == key {
			return e.args[i+1]
		}
	}

	return nil
}

// recordingLogger captures every entry the chain writes. Both LogContext
// (access logger) and ErrorContext (recovery) are exercised by a panicking
// request, so the embedded GeneralLogger must not be left to answer either
// call: it is a nil interface and would panic inside the deferred
// observation, masking the defect under test.
type recordingLogger struct {
	logger.GeneralLogger

	mu      sync.Mutex
	entries []recordedEntry
}

func (l *recordingLogger) LogContext(_ context.Context, level logger.Level, msg string, args ...any) {
	l.record(level, msg, args)
}

func (l *recordingLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	l.record(logger.ErrorLevel, msg, args)
}

func (l *recordingLogger) record(level logger.Level, msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, recordedEntry{
		level: level,
		msg:   msg,
		args:  append([]any(nil), args...),
	})
}

// find returns the entries logged under msg, in call order.
func (l *recordingLogger) find(msg string) []recordedEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]recordedEntry, 0, 1)
	for _, e := range l.entries {
		if e.msg == msg {
			out = append(out, e)
		}
	}

	return out
}

// assertPanicEntry checks the one access-log entry a panicking request must
// produce: ErrorLevel, Internal status, and the constant error text (never
// the panic value, which recovery logs with the stack).
func assertPanicEntry(t *testing.T, entries []recordedEntry, wantErr string) {
	t.Helper()

	if len(entries) != 1 {
		t.Fatalf("access log entries = %d, want 1: a panicking handler must still be logged", len(entries))
	}

	entry := entries[0]
	if entry.level != logger.ErrorLevel {
		t.Fatalf("level = %v, want %v", entry.level, logger.ErrorLevel)
	}

	if got := entry.field("status"); got != int(codes.Internal) {
		t.Fatalf("logged status = %v, want %d", got, int(codes.Internal))
	}

	if got := entry.field("error"); got != wantErr {
		t.Fatalf("logged error = %v, want the constant %q: the panic value belongs to recovery alone", got, wantErr)
	}

	for _, arg := range entry.args {
		if s, ok := arg.(string); ok && strings.Contains(s, "secret-credential-boom") {
			t.Fatalf("panic value leaked into the access log: %q", s)
		}
	}
}

// TestUnaryPanicIsObserved composes the production chain (recovery wrapping
// the access logger) directly. Before the observation was deferred, a panic
// unwound past the access logger, so the request left no access-log entry
// and no sicky_server_requests_total increment at all.
func TestUnaryPanicIsObserved(t *testing.T) {
	const method = "/svc/Panic"

	log := &recordingLogger{}
	access := NewAccessLoggerInterceptor(LoggerConfig{Logger: log})
	recovery := NewRecoveryInterceptor(LoggerConfig{Logger: log})
	info := &grpc.UnaryServerInfo{FullMethod: method}

	reqCounter := metrics.ServerRequestsTotal.WithLabelValues("grpc", method, method, codes.Internal.String())
	panicCounter := metrics.ServerPanicsTotal.WithLabelValues("grpc", method)
	beforeReq := counterValue(reqCounter)
	beforePanic := counterValue(panicCounter)

	_, err := recovery(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return access(ctx, req, info, func(ctx context.Context, req any) (any, error) {
			panic("secret-credential-boom")
		})
	})

	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("client code = %v, want Internal: recovery must stay outermost and answer the panic", got)
	}

	if delta := counterValue(reqCounter) - beforeReq; delta != 1 {
		t.Fatalf("sicky_server_requests_total{code=Internal} delta = %v, want 1: a panicking RPC must be counted", delta)
	}

	if delta := counterValue(panicCounter) - beforePanic; delta != 1 {
		t.Fatalf("sicky_server_panics_total delta = %v, want 1", delta)
	}

	assertPanicEntry(t, log.find("grpc.request"), panicUnaryLogError)
}

// TestUnarySuccessRecordsExactlyOnce guards the other direction: deferring
// the observation must not observe a successful request twice.
func TestUnarySuccessRecordsExactlyOnce(t *testing.T) {
	const method = "/svc/Success"

	log := &recordingLogger{}
	access := NewAccessLoggerInterceptor(LoggerConfig{Logger: log})
	recovery := NewRecoveryInterceptor(LoggerConfig{Logger: log})
	info := &grpc.UnaryServerInfo{FullMethod: method}

	reqCounter := metrics.ServerRequestsTotal.WithLabelValues("grpc", method, method, codes.OK.String())
	panicCounter := metrics.ServerPanicsTotal.WithLabelValues("grpc", method)
	beforeReq := counterValue(reqCounter)
	beforePanic := counterValue(panicCounter)

	resp, err := recovery(context.Background(), nil, info, func(ctx context.Context, req any) (any, error) {
		return access(ctx, req, info, func(ctx context.Context, req any) (any, error) {
			return "resp", nil
		})
	})
	if err != nil || resp != "resp" {
		t.Fatalf("passthrough broken: resp=%v err=%v", resp, err)
	}

	if delta := counterValue(reqCounter) - beforeReq; delta != 1 {
		t.Fatalf("sicky_server_requests_total{code=OK} delta = %v, want 1", delta)
	}

	if delta := counterValue(panicCounter) - beforePanic; delta != 0 {
		t.Fatalf("sicky_server_panics_total delta = %v, want 0", delta)
	}

	if entries := log.find("grpc.request"); len(entries) != 1 {
		t.Fatalf("access log entries = %d, want 1", len(entries))
	}
}

// TestStreamPanicIsObserved is the streaming counterpart of
// TestUnaryPanicIsObserved.
func TestStreamPanicIsObserved(t *testing.T) {
	const method = "/svc/StreamPanic"

	log := &recordingLogger{}
	access := NewStreamAccessLoggerInterceptor(LoggerConfig{Logger: log})
	recovery := NewStreamRecoveryInterceptor(LoggerConfig{Logger: log})
	info := &grpc.StreamServerInfo{FullMethod: method}
	ss := &stubServerStream{ctx: context.Background()}

	reqCounter := metrics.ServerRequestsTotal.WithLabelValues("grpc", method, method, codes.Internal.String())
	panicCounter := metrics.ServerPanicsTotal.WithLabelValues("grpc", method)
	beforeReq := counterValue(reqCounter)
	beforePanic := counterValue(panicCounter)

	err := recovery(nil, ss, info, func(srv any, ss grpc.ServerStream) error {
		return access(srv, ss, info, func(srv any, ss grpc.ServerStream) error {
			panic("secret-credential-boom")
		})
	})

	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("client code = %v, want Internal", got)
	}

	if delta := counterValue(reqCounter) - beforeReq; delta != 1 {
		t.Fatalf("sicky_server_requests_total{code=Internal} delta = %v, want 1: a panicking stream must be counted", delta)
	}

	if delta := counterValue(panicCounter) - beforePanic; delta != 1 {
		t.Fatalf("sicky_server_panics_total delta = %v, want 1", delta)
	}

	assertPanicEntry(t, log.find("grpc.stream"), panicStreamLogError)
}

// TestStreamSuccessRecordsExactlyOnce is the streaming non-double-count
// guard.
func TestStreamSuccessRecordsExactlyOnce(t *testing.T) {
	const method = "/svc/StreamOK"

	log := &recordingLogger{}
	access := NewStreamAccessLoggerInterceptor(LoggerConfig{Logger: log})
	recovery := NewStreamRecoveryInterceptor(LoggerConfig{Logger: log})
	info := &grpc.StreamServerInfo{FullMethod: method}
	ss := &stubServerStream{ctx: context.Background()}

	reqCounter := metrics.ServerRequestsTotal.WithLabelValues("grpc", method, method, codes.OK.String())
	panicCounter := metrics.ServerPanicsTotal.WithLabelValues("grpc", method)
	beforeReq := counterValue(reqCounter)
	beforePanic := counterValue(panicCounter)

	err := recovery(nil, ss, info, func(srv any, ss grpc.ServerStream) error {
		return access(srv, ss, info, func(srv any, ss grpc.ServerStream) error {
			return nil
		})
	})
	if err != nil {
		t.Fatalf("passthrough broken: %v", err)
	}

	if delta := counterValue(reqCounter) - beforeReq; delta != 1 {
		t.Fatalf("sicky_server_requests_total{code=OK} delta = %v, want 1", delta)
	}

	if delta := counterValue(panicCounter) - beforePanic; delta != 0 {
		t.Fatalf("sicky_server_panics_total delta = %v, want 0", delta)
	}

	if entries := log.find("grpc.stream"); len(entries) != 1 {
		t.Fatalf("access log entries = %d, want 1", len(entries))
	}
}

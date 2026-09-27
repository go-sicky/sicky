package http

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uptrace/bunrouter"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

// captureLogger returns a GeneralLogger writing JSON lines into buf so a
// test can assert on what the access logger actually recorded.
func captureLogger(buf *bytes.Buffer) logger.GeneralLogger {
	return logger.NewGeneral(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

// httpChain mirrors the production middleware order in http.go (status →
// recovery → access logger → error), skipping the transport middlewares
// that do not participate in status handling.
func httpChain(l logger.GeneralLogger, next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
	access := NewAccessLoggerMiddleware(AccessLoggerMiddlewareConfig{
		AccessLoggerConfig: DefaultAccessLogger,
		Logger:             l,
	})

	return NewStatusMiddleware()(NewRecoveryMiddleware(l)(access(NewErrorMiddleware(l)(next))))
}

// httpStatusError is an HTTPError the way a handler would return one.
type httpStatusError struct {
	msg  string
	code int
}

func (e httpStatusError) Error() string   { return e.msg }
func (e httpStatusError) StatusCode() int { return e.code }

func TestErrorHandlerAnswers500WithoutLeaking(t *testing.T) {
	var logs bytes.Buffer
	l := captureLogger(&logs)

	h := httpChain(l, func(w http.ResponseWriter, r bunrouter.Request) error {
		// Typical handler failure: the cause carries the DSN.
		return errors.New("query failed: dsn=postgres://app:s3cr3t@db:5432/app")
	})

	w := httptest.NewRecorder()
	if err := h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/x", http.NoBody))); err == nil {
		t.Fatal("middleware must keep returning the handler error to bunrouter")
	}

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "s3cr3t") || strings.Contains(w.Body.String(), "postgres") {
		t.Fatalf("internal detail leaked to client: %q", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "internal server error") {
		t.Fatalf("body = %q", w.Body.String())
	}

	// The access logger must see the real status, not a default 200.
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("access log did not record status 500: %s", logs.String())
	}

	// The message stays constant: a text handler would print an
	// attacker-supplied message verbatim. The detail rides as a field.
	if strings.Contains(logs.String(), `"msg":"query failed`) {
		t.Fatalf("the error became the log message: %s", logs.String())
	}

	if !strings.Contains(logs.String(), `"error":"query failed`) {
		t.Fatalf("the error is missing as a structured field: %s", logs.String())
	}
}

func TestErrorHandlerPanicStaysServerSide(t *testing.T) {
	var logs bytes.Buffer
	l := captureLogger(&logs)

	h := httpChain(l, func(w http.ResponseWriter, r bunrouter.Request) error {
		panic("secret-credential-boom")
	})

	w := httptest.NewRecorder()
	_ = h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/x", http.NoBody)))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "secret") || strings.Contains(body, "goroutine") {
		t.Fatalf("panic value leaked to client: %q", body)
	}

	// Stack and panic value belong to the server log only.
	if !strings.Contains(logs.String(), "secret-credential-boom") {
		t.Fatalf("panic value missing from server log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "goroutine") {
		t.Fatalf("stack missing from server log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("access log did not record status 500: %s", logs.String())
	}

	if !strings.Contains(logs.String(), `"error":"http handler panicked`) {
		t.Fatalf("the panic is missing as a structured field: %s", logs.String())
	}
}

func TestErrorHandlerKeepsStatusAlreadyWritten(t *testing.T) {
	var logs bytes.Buffer
	l := captureLogger(&logs)

	h := httpChain(l, func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("short and stout"))

		// Failure after the answer: the responder must not append.
		return errors.New("post-response failure")
	})

	w := httptest.NewRecorder()
	_ = h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/x", http.NoBody)))

	if w.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", w.Code)
	}
	if w.Body.String() != "short and stout" {
		t.Fatalf("body was overwritten: %q", w.Body.String())
	}
}

func TestErrorHandlerUsesHTTPErrorStatus(t *testing.T) {
	l := captureLogger(&bytes.Buffer{})
	h := httpChain(l, func(w http.ResponseWriter, r bunrouter.Request) error {
		return httpStatusError{msg: "no entry", code: http.StatusNotFound}
	})

	w := httptest.NewRecorder()
	_ = h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/x", http.NoBody)))

	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "no entry") {
		t.Fatalf("code = %d body = %q", w.Code, w.Body.String())
	}
}

func TestErrorHandlerUsesCodedErrorStatus(t *testing.T) {
	l := captureLogger(&bytes.Buffer{})
	h := httpChain(l, func(w http.ResponseWriter, r bunrouter.Request) error {
		// The wrapped cause must not travel to the client.
		return utils.NewCodedError(utils.CodeConflict, "", errors.New("duplicate key 42"))
	})

	w := httptest.NewRecorder()
	_ = h(w, bunrouter.NewRequest(httptest.NewRequest(http.MethodGet, "/x", http.NoBody)))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	if strings.Contains(w.Body.String(), "duplicate key") {
		t.Fatalf("wrapped cause leaked: %q", w.Body.String())
	}
}

func TestErrorHandlerMapsBodyLimitTo413(t *testing.T) {
	l := captureLogger(&bytes.Buffer{})
	chain := NewStatusMiddleware()(NewBodyLimitMiddleware(4)(NewErrorMiddleware(l)(
		func(w http.ResponseWriter, r bunrouter.Request) error {
			_, err := http.MaxBytesReader(w, r.Body, 4).Read(make([]byte, 16))
			if err == nil {
				t.Fatal("expected a body-too-large error")
			}

			return err
		},
	)))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	_ = chain(w, bunrouter.NewRequest(req))

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

// TestRouterHandlerErrorIsAnswered guards the original bunrouter defect:
// Router.ServeHTTP discards the handler error, so before the error
// responder existed a failing handler answered 200 with an empty body.
func TestRouterHandlerErrorIsAnswered(t *testing.T) {
	srv := New(
		&server.Options{Name: "http-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)

	srv.router.GET("/fail", func(w http.ResponseWriter, r bunrouter.Request) error {
		return errors.New("boom")
	})
	srv.router.GET("/panic", func(w http.ResponseWriter, r bunrouter.Request) error {
		panic("boom")
	})
	srv.router.GET("/ok", func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusAccepted)

		return nil
	})

	for _, tt := range []struct {
		path string
		want int
	}{
		{"/fail", http.StatusInternalServerError},
		{"/panic", http.StatusInternalServerError},
		{"/ok", http.StatusAccepted},
	} {
		w := httptest.NewRecorder()
		srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))
		if w.Code != tt.want {
			t.Fatalf("%s: status = %d, want %d", tt.path, w.Code, tt.want)
		}
	}

	// A 404 must still answer 404 (error responder runs for unmatched
	// routes too, and must not turn them into 500s).
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", http.NoBody))
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing route: status = %d, want 404", w.Code)
	}
}

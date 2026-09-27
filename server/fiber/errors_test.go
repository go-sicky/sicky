package fiber

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

// newTestServer builds a fiber server whose access log lands in buf.
func newTestServer(t *testing.T, buf *bytes.Buffer) *FiberServer {
	t.Helper()

	return New(
		&server.Options{
			Name: "fiber-test",
			Logger: logger.NewGeneral(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))),
		},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)
}

func do(t *testing.T, srv *FiberServer, method, path string) (code int, body string) {
	t.Helper()

	resp, err := srv.App().Test(httptest.NewRequest(method, path, http.NoBody))
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return resp.StatusCode, string(payload)
}

// TestErrorHandlerDoesNotLeakInternals: fiber's DefaultErrorHandler sent
// err.Error() verbatim, so a failing handler handed its DSN to the
// caller. The failure must still be visible as a 500 to the client and
// in the access log.
func TestErrorHandlerDoesNotLeakInternals(t *testing.T) {
	var logs bytes.Buffer
	srv := newTestServer(t, &logs)

	srv.App().Get("/boom", func(c fiber.Ctx) error {
		return errors.New("query failed: dsn=postgres://app:s3cr3t@db:5432/app")
	})

	code, body := do(t, srv, http.MethodGet, "/boom")
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if strings.Contains(body, "s3cr3t") || strings.Contains(body, "postgres") {
		t.Fatalf("internal detail leaked to client: %q", body)
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("access log did not record status 500: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "s3cr3t") {
		t.Fatalf("internal detail missing from server log: %s", logs.String())
	}
}

// TestErrorHandlerKeepsClientFacingCodes: framework and business errors
// that declare a client-visible status keep it (and the message).
func TestErrorHandlerKeepsClientFacingCodes(t *testing.T) {
	var logs bytes.Buffer
	srv := newTestServer(t, &logs)

	srv.App().Get("/gone", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusGone, "resource is gone")
	})
	srv.App().Get("/conflict", func(c fiber.Ctx) error {
		// The wrapped cause must not travel to the client.
		return utils.NewCodedError(utils.CodeConflict, "", errors.New("duplicate key 42"))
	})

	if code, body := do(t, srv, http.MethodGet, "/gone"); code != fiber.StatusGone || !strings.Contains(body, "resource is gone") {
		t.Fatalf("gone: code = %d body = %q", code, body)
	}

	if code, body := do(t, srv, http.MethodGet, "/conflict"); code != http.StatusConflict || !strings.Contains(body, "conflict") {
		t.Fatalf("conflict: code = %d body = %q", code, body)
	} else if strings.Contains(body, "duplicate key") {
		t.Fatalf("wrapped cause leaked: %q", body)
	}
}

// TestPanicAnsweredGenerically: fiber's default panic handler returned
// the panic value, which the error handler then shipped to the client.
func TestPanicAnsweredGenerically(t *testing.T) {
	var logs bytes.Buffer
	srv := newTestServer(t, &logs)

	srv.App().Get("/panic", func(c fiber.Ctx) error {
		panic("secret-credential-boom")
	})

	before := testutil.ToFloat64(metrics.ServerPanicsTotal.WithLabelValues("fiber", http.MethodGet))

	code, body := do(t, srv, http.MethodGet, "/panic")
	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", code)
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "goroutine") {
		t.Fatalf("panic value leaked to client: %q", body)
	}

	// The panic value and the stack belong to the server log only.
	if !strings.Contains(logs.String(), "secret-credential-boom") {
		t.Fatalf("panic value missing from server log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "goroutine") {
		t.Fatalf("stack missing from server log: %s", logs.String())
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("access log did not record status 500: %s", logs.String())
	}

	if after := testutil.ToFloat64(metrics.ServerPanicsTotal.WithLabelValues("fiber", http.MethodGet)); after-before != 1 {
		t.Fatalf("panic counter delta = %v, want 1", after-before)
	}
}

// TestFrameworkNotFoundKeepsStatus: the framework's own 404/405 answers
// travel through the same error handler and must not collapse into 500s.
func TestFrameworkNotFoundKeepsStatus(t *testing.T) {
	var logs bytes.Buffer
	srv := newTestServer(t, &logs)

	srv.App().Get("/ping", func(c fiber.Ctx) error {
		return nil
	})

	if code, _ := do(t, srv, http.MethodGet, "/missing"); code != http.StatusNotFound {
		t.Fatalf("missing route: status = %d, want 404", code)
	}

	if code, _ := do(t, srv, http.MethodPost, "/ping"); code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method: status = %d, want 405", code)
	}

	if !strings.Contains(logs.String(), `"status":404`) || !strings.Contains(logs.String(), `"status":405`) {
		t.Fatalf("access log missed 404/405: %s", logs.String())
	}
}

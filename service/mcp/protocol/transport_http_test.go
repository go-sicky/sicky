package protocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthorizeRequestRules(t *testing.T) {
	tr := NewHTTPTransport(":3000")

	newReq := func(remote, host, origin, auth string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://"+host+"/mcp/message", http.NoBody)
		r.RemoteAddr = remote
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}

		return r
	}

	// No token: loopback + loopback Host + no foreign Origin.
	if code, _ := tr.authorizeRequest(newReq("127.0.0.1:40000", "localhost:3000", "", "")); code != http.StatusOK {
		t.Fatalf("loopback request = %d, want 200", code)
	}

	// Remote peer without a token: refused.
	if code, _ := tr.authorizeRequest(newReq("192.168.1.9:40000", "localhost:3000", "", "")); code != http.StatusForbidden {
		t.Fatalf("remote request = %d, want 403", code)
	}

	// DNS rebinding: still loopback, but the Host is the attacker's
	// domain - the only signal left that distinguishes the two.
	if code, _ := tr.authorizeRequest(newReq("127.0.0.1:40000", "evil.example:3000", "", "")); code != http.StatusForbidden {
		t.Fatalf("rebound host = %d, want 403", code)
	}

	// Cross-site browser request: refused even on loopback.
	if code, _ := tr.authorizeRequest(newReq("127.0.0.1:40000", "localhost:3000", "http://evil.example", "")); code != http.StatusForbidden {
		t.Fatalf("cross-site origin = %d, want 403", code)
	}

	// Same-origin browser request is fine.
	if code, _ := tr.authorizeRequest(newReq("127.0.0.1:40000", "localhost:3000", "http://localhost:3000", "")); code != http.StatusOK {
		t.Fatalf("same-origin = %d, want 200", code)
	}

	// With a token the remote peer is accepted, wrong token is not.
	tr.AuthToken = "s3cr3t"
	if code, _ := tr.authorizeRequest(newReq("192.168.1.9:40000", "api.internal:3000", "", "Bearer s3cr3t")); code != http.StatusOK {
		t.Fatalf("remote with token = %d, want 200", code)
	}

	if code, _ := tr.authorizeRequest(newReq("192.168.1.9:40000", "api.internal:3000", "", "Bearer wrong")); code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", code)
	}
}

// TestHTTPTransportSessionIsolation: responses must reach only the
// client whose message is being processed - a shared buffer let any
// connected SSE stream read (and drain) another client's answers.
func TestHTTPTransportSessionIsolation(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() {
		if err := tr.Stop(); err != nil {
			t.Errorf("stop: %v", err)
		}
	}()

	base := "http://" + tr.ln.Addr().String()
	client := &http.Client{Timeout: 10 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Client A opens the stream and learns its session.
	sseA, sessionA := openSSE(t, client, ctx, base)
	defer func() {
		_ = sseA.Close()
	}()

	if sessionA == "" {
		t.Fatal("endpoint event did not announce a session")
	}

	// Client B connects too: it must get its own session.
	sseB, sessionB := openSSE(t, client, ctx, base)
	defer func() {
		_ = sseB.Close()
	}()

	if sessionB == "" || sessionB == sessionA {
		t.Fatalf("sessions must be distinct: %q vs %q", sessionA, sessionB)
	}

	// A message without a session cannot be delivered anywhere.
	resp, err := client.Post(base+"/mcp/message", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatalf("post without session: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("post without session = %d, want 409", resp.StatusCode)
	}

	// A posts a message; the server answers it.
	post(t, client, base, sessionA, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, http.StatusAccepted)

	msg, err := tr.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.Contains(string(msg), `"method":"ping"`) {
		t.Fatalf("read message = %q", msg)
	}

	if err := tr.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Only A's stream carries the response.
	if line := readLine(t, ctx, sseA); !strings.Contains(line, `"id":1`) {
		t.Fatalf("stream A got %q, want the response", line)
	}

	if line := peekLine(t, sseB); strings.Contains(line, `"id":1`) {
		t.Fatalf("stream B received another client's response: %q", line)
	}
}

func TestHTTPTransportRejectsOversizedMessage(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")
	tr.messages = make(chan []byte, 1)
	tr.done = make(chan struct{})
	tr.sessions["s"] = &sseSession{out: make(chan []byte, 1)}

	body := strings.Repeat("a", maxMessageBytes+1024)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp/message?session=s", strings.NewReader(body))
	w := httptest.NewRecorder()

	tr.handleMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized message = %d, want 400", w.Code)
	}

	if len(tr.messages) != 0 {
		t.Fatal("oversized message reached the server loop")
	}
}

func TestHTTPTransportStopWakesReader(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:0")

	read := make(chan error, 1)
	go func() {
		_, err := tr.Read()
		read <- err
	}()

	if err := tr.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	select {
	case err := <-read:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Read after Stop = %v, want io.EOF", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read did not wake up on Stop")
	}
}

// openSSE opens a stream and returns its body plus the announced session.
func openSSE(t *testing.T, client *http.Client, ctx context.Context, base string) (body io.ReadCloser, session string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/mcp", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("sse status = %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	event := readLine(t, ctx, reader)
	data := readLine(t, ctx, reader)
	if !strings.Contains(event, "event: endpoint") {
		_ = resp.Body.Close()
		t.Fatalf("first event = %q, want the endpoint event", event)
	}

	sid := strings.TrimPrefix(strings.TrimSpace(data), "data: ")
	if idx := strings.Index(sid, "session="); idx >= 0 {
		sid = sid[idx+len("session="):]
	}

	return resp.Body, sid
}

func post(t *testing.T, client *http.Client, base, session, payload string, want int) {
	t.Helper()

	url := base + "/mcp/message?session=" + session
	resp, err := client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("post = %d (want %d): %s", resp.StatusCode, want, body)
	}
}

// readLine reads one line, honoring the context deadline.
func readLine(t *testing.T, ctx context.Context, r io.Reader) string {
	t.Helper()

	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)
	reader := bufio.NewReader(r)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("read line: %v", res.err)
		}

		return res.line
	case <-ctx.Done():
		t.Fatalf("read line: %v", ctx.Err())
	}

	return ""
}

// peekLine reports the next line without blocking on a slow stream.
func peekLine(t *testing.T, r io.Reader) string {
	t.Helper()

	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)
	reader := bufio.NewReader(r)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case res := <-ch:
		return res.line
	case <-time.After(500 * time.Millisecond):
		return ""
	}
}

// TestAuthorizeAcceptsListedHost: a deployment that fronts the transport
// with a reverse proxy lists its name; loopback alone is not enough when
// the Host does not look local.
func TestAuthorizeAcceptsListedHost(t *testing.T) {
	tr := NewHTTPTransport("127.0.0.1:3000")
	tr.AllowedHosts = []string{"api.example.com"}

	newReq := func(host string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://"+host+"/mcp/message", http.NoBody)
		r.RemoteAddr = "127.0.0.1:40000"
		r.Host = host

		return r
	}

	if code, _ := tr.authorizeRequest(newReq("api.example.com:3000")); code != http.StatusOK {
		t.Fatalf("listed host = %d, want 200", code)
	}

	if code, _ := tr.authorizeRequest(newReq("evil.example:3000")); code != http.StatusForbidden {
		t.Fatalf("unlisted host = %d, want 403", code)
	}
}

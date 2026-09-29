/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file transport_http.go
 * @package protocol
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package protocol

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-sicky/sicky/utils"
)

/* {{{ [HTTPTransport] */

// maxMessageBytes caps a single MCP message body. Without it io.ReadAll
// hands an unbounded allocation to whoever can reach the port.
const maxMessageBytes = 1 << 20 // 1 MiB

// sseOutboxSize bounds the responses queued for one SSE client; a client
// that stops reading is dropped instead of growing the heap.
const sseOutboxSize = 64

const (
	// msgForbidden is the body returned alongside an access denial; it
	// deliberately says nothing about which rule failed.
	msgForbidden = "forbidden"
	// schemeHTTPS is the forwarded/plain scheme marker used by the
	// origin comparison.
	schemeHTTPS = "https"
)

// sseSession is one connected SSE client. Responses are queued per
// session: a shared buffer let any connected client read (and drain)
// another client's answers.
type sseSession struct {
	out chan []byte
}

// inbound is one client message together with the session that sent it.
// The session travels with the message instead of being parked on the
// transport: a single "current session" field is overwritten by every
// handler goroutine as soon as two clients post concurrently, so the
// answer to the first message is written to the second client's stream.
type inbound struct {
	body []byte
	sess *sseSession
}

// HTTPTransport is a protocol component.
//
// Access rules (checked on both endpoints): a valid bearer token always
// wins; without one the peer must be loopback (else403), the Host header
// must be a loopback name or an explicitly allowed one (defeats DNS
// rebinding, which lands on loopback), and a browser Origin must match
// the request (defeats CSRF from another site).
type HTTPTransport struct {
	addr string

	// AuthToken, when set, must be presented as `Authorization: Bearer
	// <token>`. Leave empty for loopback-only access.
	AuthToken string
	// AllowedHosts adds Host header values (host or host:port) accepted
	// without a token.
	AllowedHosts []string

	messages  chan inbound
	done      chan struct{}
	closeOnce sync.Once

	mu       sync.Mutex
	sessions map[string]*sseSession
	closed   bool
	srv      *http.Server
	ln       net.Listener

	// current is the session owning the message the serve loop is
	// handling right now. It is written by Read (the serve goroutine)
	// and read by Write (the same goroutine, before its next Read), so
	// it needs no lock; the serve loop is serial.
	current *sseSession
}

// NewHTTPTransport creates a new HTTPTransport.
func NewHTTPTransport(addr string) *HTTPTransport {
	return &HTTPTransport{
		addr:     addr,
		messages: make(chan inbound, 256),
		done:     make(chan struct{}),
		sessions: make(map[string]*sseSession),
	}
}

// Start starts the component. The listener is created synchronously so a
// port already in use fails the start instead of being reported as
// success from a background goroutine.
func (t *HTTPTransport) Start() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()

		return errors.New("mcp http transport: already stopped")
	}
	t.mu.Unlock()

	ln, err := net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("mcp http transport listen on %s: %w", t.addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", t.handleSSE)
	mux.HandleFunc("/mcp/message", t.handleMessage)

	t.mu.Lock()
	t.ln = ln
	t.srv = &http.Server{
		Handler:      t.authorize(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	t.mu.Unlock()

	if t.AuthToken == "" && externalBind(t.addr) {
		// Not fatal (the transport is still loopback-only), but the
		// operator most likely expected remote access to work.
		fmt.Printf("sicky mcp: listening on %s without an auth token; only loopback clients are accepted\n", ln.Addr().String())
	}

	go func() {
		if err := t.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()

			if !closed {
				fmt.Printf("HTTP transport error: %s\n", err.Error())
			}
		}
	}()

	return nil
}

// Stop stops the component and releases resources.
func (t *HTTPTransport) Stop() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()

		return nil
	}

	t.closed = true
	srv := t.srv
	t.mu.Unlock()

	// The messages channel is deliberately not closed: a handler that
	// passed the closed check must never race into a send on a closed
	// channel. Readers wake up on done instead.
	t.closeOnce.Do(func() { close(t.done) })

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = srv.Shutdown(ctx)
	}

	return nil
}

// Read reads the next client message and remembers which session sent
// it, so the matching Write goes back to that session.
func (t *HTTPTransport) Read() ([]byte, error) {
	select {
	case msg := <-t.messages:
		// Bind the target here, on the serve goroutine, not in the
		// handler that enqueued it: by the time this message is
		// processed another client may already have posted.
		t.current = msg.sess

		return msg.body, nil
	case <-t.done:
		return nil, io.EOF
	}
}

// Write delivers a server response to the client that owns the message
// currently being processed.
func (t *HTTPTransport) Write(data []byte) error {
	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()

	if closed {
		return io.ErrClosedPipe
	}

	// Read and Write run on the same (serve) goroutine with no Read in
	// between, so this needs no lock.
	target := t.current
	if target == nil {
		// Nobody is waiting: dropping beats writing into a buffer some
		// other client would drain.
		return nil
	}

	frame := make([]byte, 0, len(data)+8)
	frame = append(frame, "data: "...)
	frame = append(frame, data...)
	frame = append(frame, "\n\n"...)

	select {
	case target.out <- frame:
	case <-t.done:
		return io.ErrClosedPipe
	default:
		// The client stopped reading: drop rather than block the
		// server loop or grow without bound.
	}

	return nil
}

// authorize wraps the mux with the access rules documented on
// HTTPTransport.
func (t *HTTPTransport) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code, msg := t.authorizeRequest(r); code != http.StatusOK {
			if msg != "" {
				http.Error(w, msg, code)
			} else {
				w.WriteHeader(code)
			}

			return
		}

		next.ServeHTTP(w, r)
	})
}

// authorizeRequest evaluates the access rules and returns an HTTP status
// (200 when the request may proceed).
func (t *HTTPTransport) authorizeRequest(r *http.Request) (status int, msg string) {
	if t.AuthToken != "" {
		// Compare equal-length digests: no length oracle on the token.
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		want := sha256.Sum256([]byte("Bearer " + t.AuthToken))
		if subtle.ConstantTimeCompare(got[:], want[:]) == 1 {
			return http.StatusOK, ""
		}

		return http.StatusUnauthorized, "unauthorized"
	}

	// No token: everything below must hold at once.
	if !isLoopbackPeer(r.RemoteAddr) {
		return http.StatusForbidden, msgForbidden
	}

	// A rebound hostname still connects to loopback, so the Host header
	// is what tells the two apart.
	if !t.hostAllowed(r.Host) {
		return http.StatusForbidden, msgForbidden
	}

	// A cross-site POST is the CSRF shape: the browser attaches the
	// request but cannot read the answer.
	if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r) {
		return http.StatusForbidden, msgForbidden
	}

	return http.StatusOK, ""
}

// hostAllowed reports whether a Host header is acceptable without a
// token.
func (t *HTTPTransport) hostAllowed(host string) bool {
	normalized := utils.NormalizeHost(host)
	if normalized == "" {
		return false
	}

	for _, allowed := range t.AllowedHosts {
		if utils.NormalizeHost(allowed) == normalized {
			return true
		}
	}

	// What a browser resolves a rebound name to on this machine.
	if utils.IsLoopbackHost(normalized) {
		return true
	}

	// The address we are bound to is a legitimate Host when it is not a
	// wildcard.
	if bound := utils.NormalizeHost(boundHost(t.addr)); bound != "" && bound == normalized {
		return true
	}

	return false
}

// handleSSE serves one client stream. The first event announces the
// message endpoint together with the session that owns its responses.
func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)

		return
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()

		http.Error(w, "server closed", http.StatusServiceUnavailable)

		return
	}

	sid := hex.EncodeToString(utils.RandomHex(16))
	sess := &sseSession{out: make(chan []byte, sseOutboxSize)}
	t.sessions[sid] = sess
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.sessions, sid)
		t.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Announce where this session sends messages.
	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: %s?session=%s\n\n", "/mcp/message", sid); err != nil {
		return
	}

	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.done:
			return
		case frame := <-sess.out:
			if _, err := w.Write(frame); err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

// handleMessage accepts one client message and routes its responses to
// the session that sent it.
func (t *HTTPTransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	sid := r.URL.Query().Get("session")
	if sid == "" {
		sid = r.Header.Get("Mcp-Session-Id")
	}

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()

		http.Error(w, "server closed", http.StatusServiceUnavailable)

		return
	}

	sess := t.sessions[sid]
	if sess == nil {
		t.mu.Unlock()

		// An unknown session means the stream was never opened (or has
		// ended): answering 202 would silently swallow the message.
		http.Error(w, "unknown session", http.StatusConflict)

		return
	}

	t.mu.Unlock()

	// Bounded read: the body comes from the network.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMessageBytes))
	defer func() {
		_ = r.Body.Close()
	}()

	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}

	select {
	case t.messages <- inbound{body: body, sess: sess}:
	case <-t.done:
		http.Error(w, "server closed", http.StatusServiceUnavailable)

		return
	default:
		// Back-pressure instead of unbounded queueing.
		http.Error(w, "too many pending messages", http.StatusTooManyRequests)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// isLoopbackPeer reports whether a net.Conn RemoteAddr is loopback.
func isLoopbackPeer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}

	ip := net.ParseIP(strings.TrimSpace(host))

	return ip != nil && ip.IsLoopback()
}

// sameOrigin reports whether an Origin header matches the request's own
// scheme and host.
func sameOrigin(origin string, r *http.Request) bool {
	u := strings.TrimSuffix(strings.ToLower(origin), "/")
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == schemeHTTPS {
		scheme = "https"
	}

	if strings.HasPrefix(u, schemeHTTPS+"://") {
		scheme = schemeHTTPS
	} else if !strings.HasPrefix(u, "http://") {
		return false
	}

	return u == scheme+"://"+strings.ToLower(r.Host)
}

// boundHost returns the host part of a listen address ("" for wildcards).
func boundHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}

	return host
}

// externalBind reports whether addr listens beyond this machine.
func externalBind(addr string) bool {
	host := boundHost(addr)
	if host == "" {
		return true
	}

	ip := net.ParseIP(host)

	return ip == nil || !ip.IsLoopback()
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

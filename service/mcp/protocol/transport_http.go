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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

/* {{{ [HTTPTransport] */

type HTTPTransport struct {
	addr     string
	messages chan []byte
	mu       sync.Mutex
	buf      bytes.Buffer
	closed   bool
}

func NewHTTPTransport(addr string) *HTTPTransport {
	return &HTTPTransport{
		addr:     addr,
		messages: make(chan []byte, 256),
	}
}

func (t *HTTPTransport) Start() error {
	t.closed = false

	http.HandleFunc("/mcp", t.handleSSE)
	http.HandleFunc("/mcp/message", t.handleMessage)

	go func() {
		if err := http.ListenAndServe(t.addr, nil); err != nil {
			t.mu.Lock()

			defer t.mu.Unlock()

			if !t.closed {
				fmt.Printf("HTTP transport error: %s\n", err.Error())
			}
		}
	}()

	return nil
}

func (t *HTTPTransport) Stop() error {
	t.mu.Lock()

	defer t.mu.Unlock()

	t.closed = true
	close(t.messages)

	return nil
}

func (t *HTTPTransport) Read() ([]byte, error) {
	msg, ok := <-t.messages
	if !ok {
		return nil, io.EOF
	}

	return msg, nil
}

func (t *HTTPTransport) Write(data []byte) error {
	t.mu.Lock()

	defer t.mu.Unlock()

	if t.closed {
		return io.ErrClosedPipe
	}

	t.buf.Reset()
	t.buf.WriteString("data: ")
	t.buf.Write(data)
	t.buf.WriteString("\n\n")

	return nil
}

func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)

		return
	}

	ticker := time.NewTicker(100 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-ticker.C:
			t.mu.Lock()
			data := t.buf.Bytes()
			t.buf.Reset()
			t.mu.Unlock()

			if len(data) > 0 {
				w.Write(data)
				flusher.Flush()
			}
		}
	}
}

func (t *HTTPTransport) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}

	defer r.Body.Close()

	t.mu.Lock()
	closed := t.closed
	t.mu.Unlock()

	if closed {
		http.Error(w, "server closed", http.StatusServiceUnavailable)

		return
	}

	t.messages <- body
	w.WriteHeader(http.StatusAccepted)
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

/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

/**
 * @file client_test.go
 * @package client
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package client

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// stubClient is a minimal implementation used to exercise the package-level
// registry; Call is not part of it because that is what the tests below pin.
type stubClient struct{ id string }

func (s *stubClient) Context() context.Context { return context.Background() }
func (s *stubClient) Options() *Options        { return (&Options{}).Ensure() }
func (s *stubClient) Connect() error           { return nil }
func (s *stubClient) Disconnect() error        { return nil }
func (s *stubClient) Call() error              { return ErrClientNotImplemented }
func (s *stubClient) String() string           { return "stub" }
func (s *stubClient) Name() string             { return "stub" }
func (s *stubClient) ID() uuid.UUID            { return uuid.MustParse(s.id) }

// Client.Call exists only to satisfy the interface: its signature carries no
// target, so no implementation could act on it. All five returned nil, which
// told a caller holding only client.Client that its RPC had been delivered —
// the same false-success hazard broker.Publish was hardened against when it
// started returning ErrNotInitialized. The protocol-specific methods
// (Invoke/NewStream, Do) are the real paths and are unaffected.
func TestErrClientNotImplementedIsMatchable(t *testing.T) {
	if !errors.Is(ErrClientNotImplemented, ErrClientNotImplemented) {
		t.Fatal("sentinel must match itself under errors.Is")
	}

	if errors.Is(ErrClientNotImplemented, errors.New("client: Call is not implemented, use the protocol-specific method")) {
		t.Fatal("a distinct error value must not match the sentinel")
	}
}

func TestStubClientCallReportsNotImplemented(t *testing.T) {
	var c Client = &stubClient{id: "00000000-0000-0000-0000-000000000001"}

	if err := c.Call(); !errors.Is(err, ErrClientNotImplemented) {
		t.Fatalf("Call() = %v, want ErrClientNotImplemented", err)
	}
}

func TestRegistryFirstWinsAndClear(t *testing.T) {
	Clear()
	t.Cleanup(Clear)

	first := &stubClient{id: "00000000-0000-0000-0000-00000000000a"}
	second := &stubClient{id: "00000000-0000-0000-0000-00000000000b"}

	Set(first, second)

	if Default() != first {
		t.Error("first registered must become the default")
	}

	if Get(first.ID()) != first || Get(second.ID()) != second {
		t.Error("both must be retrievable by ID")
	}

	if got := len(Clients()); got != 2 {
		t.Errorf("Clients() = %d, want 2", got)
	}

	Clear()

	if Default() != nil {
		t.Error("Clear must drop the default")
	}

	if got := len(Clients()); got != 0 {
		t.Errorf("Clients() after Clear = %d, want 0", got)
	}
}

package tcp

import (
	"sync"
	"testing"

	"github.com/go-sicky/sicky/server"
)

// TestHandleConcurrentNoLostRegistration: Handle used to CAS against a
// second Load of the pointer, so concurrent registrations could succeed
// on top of each other and drop a handler whose "registered" log line
// had already been written.
func TestHandleConcurrentNoLostRegistration(t *testing.T) {
	srv := New(
		&server.Options{Name: "tcp-test"},
		&Config{Network: "tcp", Address: "127.0.0.1:0"},
	)

	const (
		writers = 8
		perW    = 32
	)

	var wg sync.WaitGroup
	for range writers {
		// WaitGroup.Go accounts for the Add itself; a separate Add here
		// would leave the counter permanently positive and hang Wait.
		wg.Go(func() {
			for range perW {
				srv.Handle(noopHandler{})
			}
		})
	}

	wg.Wait()

	if got := len(srv.snapshotHandlers()); got != writers*perW {
		t.Fatalf("registered %d handlers, want %d", got, writers*perW)
	}
}

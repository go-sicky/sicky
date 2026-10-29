/**
 * @file bun_tx_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/23/2026
 * @brief Tests for Bun context-carried executors.
 */

package infra

import (
	"context"
	"errors"
	"testing"

	"github.com/uptrace/bun"
)

func TestWithTxContext(t *testing.T) {
	tx := &bun.Tx{}
	ctx := WithBunTx(context.Background(), tx)

	if got := BunDB(ctx); got != tx {
		t.Fatalf("BunDB(ctx) = %T, want the injected transaction", got)
	}
}

func TestDBFallsBackToBun(t *testing.T) {
	old := Bun
	Bun = nil
	defer func() { Bun = old }()
	if got := BunDB(context.Background()); got != nil {
		t.Fatalf("BunDB(context.Background()) = %T, want nil", got)
	}
}

func TestRunInTxWithoutBun(t *testing.T) {
	old := Bun
	Bun = nil
	defer func() { Bun = old }()

	called := false
	err := RunInBunTx(context.Background(), nil, func(context.Context) error {
		called = true
		return nil
	})

	if !errors.Is(err, ErrBunUnavailable) {
		t.Fatalf("RunInTx error = %v, want ErrBunUnavailable", err)
	}
	if called {
		t.Fatal("RunInTx called the callback without a database")
	}
}

// A typed nil *bun.DB stored in the bun.IDB interface is not equal to nil.
// WithBunTx must not put it in the context, or BunDB hands a nil pointer to
// the next caller.
func TestWithTxTypedNil(t *testing.T) {
	old := Bun
	Bun = nil
	defer func() { Bun = old }()

	var db *bun.DB
	ctx := WithBunTx(context.Background(), db)

	if got := ctx.Value(bunTxContextKey{}); got != nil {
		t.Fatalf("WithBunTx stored a typed nil: %T", got)
	}

	if got := BunDB(ctx); got != nil {
		t.Fatalf("BunDB(ctx) = %T, want nil", got)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */

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

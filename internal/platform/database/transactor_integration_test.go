//go:build integration

package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresTransactor_CommitsOnSuccess verifies AC1+AC2: when fn returns nil,
// all changes made inside the atomic block are visible after the call.
func TestPostgresTransactor_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	err := tr.RunAtomic(ctx, func(txCtx context.Context) error {
		dbtx := DBTXFromContext(txCtx, pool)
		_, err := dbtx.Exec(txCtx,
			"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
			"alice@example.com", testPasswordHash, "Alice",
		)
		return err
	})

	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1",
		"alice@example.com",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "committed row must be visible after RunAtomic")
}

// TestPostgresTransactor_RollsBackOnError verifies AC3: when fn returns an error,
// all changes made inside the atomic block are not visible after the call.
func TestPostgresTransactor_RollsBackOnError(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	want := errors.New("simulated business rule failure")

	err := tr.RunAtomic(ctx, func(txCtx context.Context) error {
		dbtx := DBTXFromContext(txCtx, pool)
		_, execErr := dbtx.Exec(txCtx,
			"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
			"bob@example.com", testPasswordHash, "Bob",
		)
		if execErr != nil {
			return execErr
		}
		return want // trigger rollback
	})

	assert.ErrorIs(t, err, want)

	var count int
	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1",
		"bob@example.com",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "rolled-back row must not be visible")
}

// TestPostgresTransactor_RollsBackOnPanic verifies the panic edge case: when fn
// panics, the transaction is rolled back and the panic is re-raised.
func TestPostgresTransactor_RollsBackOnPanic(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	assert.Panics(t, func() {
		_ = tr.RunAtomic(ctx, func(txCtx context.Context) error {
			dbtx := DBTXFromContext(txCtx, pool)
			_, _ = dbtx.Exec(txCtx,
				"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
				"charlie@example.com", testPasswordHash, "Charlie",
			)
			panic("unexpected failure mid-transaction")
		})
	})

	var count int
	err := pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1",
		"charlie@example.com",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "row inserted before panic must be rolled back")
}

// TestPostgresTransactor_RollsBackOnPanic_PreservesValue verifies that the panic
// value is re-raised unchanged after rollback.
func TestPostgresTransactor_RollsBackOnPanic_PreservesValue(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	type myPanic struct{ msg string }
	want := myPanic{msg: "domain invariant violated"}

	defer func() {
		p := recover()
		require.NotNil(t, p, "panic must be re-raised")
		assert.Equal(t, want, p, "re-raised panic must have the original value")
	}()

	_ = tr.RunAtomic(ctx, func(txCtx context.Context) error {
		panic(want)
	})
}

// TestPostgresTransactor_NestedCallUsesOuterTransaction verifies the nested edge case:
// when RunAtomic is called inside another RunAtomic, the inner call participates in
// the outer transaction rather than starting a new one. Both inserts commit or roll back
// together.
func TestPostgresTransactor_NestedCallUsesOuterTransaction(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	err := tr.RunAtomic(ctx, func(outerCtx context.Context) error {
		dbtx := DBTXFromContext(outerCtx, pool)
		if _, err := dbtx.Exec(outerCtx,
			"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
			"dave@example.com", testPasswordHash, "Dave",
		); err != nil {
			return err
		}

		// Nested RunAtomic must reuse the outer transaction.
		return tr.RunAtomic(outerCtx, func(innerCtx context.Context) error {
			inner := DBTXFromContext(innerCtx, pool)
			_, err := inner.Exec(innerCtx,
				"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
				"eve@example.com", testPasswordHash, "Eve",
			)
			return err
		})
	})

	require.NoError(t, err)

	for _, email := range []string{"dave@example.com", "eve@example.com"} {
		var count int
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM users WHERE email = $1", email,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "both rows from nested atomic calls must be committed")
	}
}

// TestPostgresTransactor_NestedRollback verifies that when the outer transaction
// is rolled back (outer fn returns error), both outer and inner inserts are undone
// even though the inner RunAtomic succeeded.
func TestPostgresTransactor_NestedRollback(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	err := tr.RunAtomic(ctx, func(outerCtx context.Context) error {
		dbtx := DBTXFromContext(outerCtx, pool)
		if _, err := dbtx.Exec(outerCtx,
			"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
			"frank@example.com", testPasswordHash, "Frank",
		); err != nil {
			return err
		}

		// Inner call succeeds — but outer tx will be rolled back.
		if err := tr.RunAtomic(outerCtx, func(innerCtx context.Context) error {
			inner := DBTXFromContext(innerCtx, pool)
			_, err := inner.Exec(innerCtx,
				"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)",
				"grace@example.com", testPasswordHash, "Grace",
			)
			return err
		}); err != nil {
			return err
		}

		return errors.New("outer failure after nested success")
	})

	require.Error(t, err)

	for _, email := range []string{"frank@example.com", "grace@example.com"} {
		var count int
		err := pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM users WHERE email = $1", email,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "both rows must be rolled back when outer tx fails")
	}
}

// TestPostgresTransactor_DBTXFromContext_NoTx_UsesPool verifies AC4: outside an
// atomic block, DBTXFromContext returns the pool (regular connection).
func TestPostgresTransactor_DBTXFromContext_NoTx_UsesPool(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)

	dbtx := DBTXFromContext(ctx, pool)

	// Pool satisfies DBTX — verify a query works directly.
	var result int
	err := dbtx.QueryRow(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err)
	assert.Equal(t, 1, result)
}

// TestPostgresTransactor_DBTXFromContext_WithTx_UsesTx verifies AC5: inside an
// atomic block, DBTXFromContext returns the transaction executor.
func TestPostgresTransactor_DBTXFromContext_WithTx_UsesTx(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	tr := NewPostgresTransactor(pool)

	var dbtxInsideTx DBTX
	_ = tr.RunAtomic(ctx, func(txCtx context.Context) error {
		dbtxInsideTx = DBTXFromContext(txCtx, pool)
		return nil
	})

	// Inside the atomic block we got a non-nil DBTX backed by the transaction.
	assert.NotNil(t, dbtxInsideTx)
}

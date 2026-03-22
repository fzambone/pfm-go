package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertion: FakeTransactor must satisfy the Transactor interface.
var _ Transactor = (*FakeTransactor)(nil)

// TestFakeTransactor_CommitsOnSuccess verifies that when fn returns nil,
// RunAtomic reports committed and returns no error.
func TestFakeTransactor_CommitsOnSuccess(t *testing.T) {
	ft := NewFakeTransactor()

	err := ft.RunAtomic(context.Background(), func(_ context.Context) error {
		return nil
	})

	require.NoError(t, err)
	assert.True(t, ft.CommittedLastCall(), "committed must be true when fn returns nil")
}

// TestFakeTransactor_RollsBackOnError verifies that when fn returns an error,
// RunAtomic reports not committed and returns the same error.
func TestFakeTransactor_RollsBackOnError(t *testing.T) {
	ft := NewFakeTransactor()
	want := errors.New("simulated failure")

	err := ft.RunAtomic(context.Background(), func(_ context.Context) error {
		return want
	})

	assert.ErrorIs(t, err, want)
	assert.False(t, ft.CommittedLastCall(), "committed must be false when fn returns an error")
}

// TestFakeTransactor_PassesContextThrough verifies that the derived context
// received by fn is the same context passed to RunAtomic (no substitution).
func TestFakeTransactor_PassesContextThrough(t *testing.T) {
	ft := NewFakeTransactor()

	type ctxKey struct{}
	outer := context.WithValue(context.Background(), ctxKey{}, "marker")

	var gotVal any
	require.NoError(t, ft.RunAtomic(outer, func(ctx context.Context) error {
		gotVal = ctx.Value(ctxKey{})
		return nil
	}))

	assert.Equal(t, "marker", gotVal, "fn must receive the original context unchanged")
}

// TestTxFromContext_WhenNotSet_ReturnsFalse verifies that a plain context
// carries no transaction.
func TestTxFromContext_WhenNotSet_ReturnsFalse(t *testing.T) {
	tx, ok := TxFromContext(context.Background())

	assert.False(t, ok)
	assert.Nil(t, tx)
}

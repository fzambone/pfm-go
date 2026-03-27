package ledger_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/domain/ledger"
)

// TestRepository_ComposesReaderAndWriter is a compile-time verification that
// Repository composes LedgerReader (2 methods) and LedgerWriter (1 method).
// If the interface changes, the stub breaks and this test fails to compile.
func TestRepository_ComposesReaderAndWriter(t *testing.T) {
	var _ ledger.Repository = (*repoStub)(nil)
	assert.True(t, true, "Repository composes LedgerReader and LedgerWriter")
}

// repoStub satisfies ledger.Repository at compile time.
type repoStub struct{}

func (repoStub) GetBalance(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

func (repoStub) GetTransactionHistory(_ context.Context, _ uuid.UUID, _ ledger.HistoryQuery) ([]ledger.TransactionWithEntries, error) {
	return nil, nil
}

func (repoStub) PostTransaction(_ context.Context, _ uuid.UUID, _ ledger.PostTransactionInput, _ uuid.UUID) (ledger.Transaction, []ledger.Entry, error) {
	return ledger.Transaction{}, nil, nil
}

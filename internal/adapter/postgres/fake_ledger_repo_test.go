package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

var (
	fakeLedgerHouseholdID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	fakeLedgerDebitAcct   = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	fakeLedgerCreditAcct  = uuid.MustParse("00000000-0000-0000-0000-000000000021")
	fakeLedgerCallerID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	fakeLedgerFixedTime   = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
)

func balancedInput() ledger.PostTransactionInput {
	return ledger.PostTransactionInput{
		Description:     "Test",
		TransactionDate: fakeLedgerFixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: fakeLedgerDebitAcct, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: fakeLedgerCreditAcct, EntryType: types.EntryTypeCredit, Amount: 10000},
		},
	}
}

// ---------------------------------------------------------------------------
// PostTransaction
// ---------------------------------------------------------------------------

func TestFakeLedgerRepo_PostTransaction_BalancedSucceeds(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	tx, entries, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tx.ID)
	assert.Equal(t, "Test", tx.Description)
	assert.Len(t, entries, 2)
}

func TestFakeLedgerRepo_PostTransaction_UnbalancedFails(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()
	input := ledger.PostTransactionInput{
		Description:     "Unbalanced",
		TransactionDate: fakeLedgerFixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: fakeLedgerDebitAcct, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: fakeLedgerCreditAcct, EntryType: types.EntryTypeCredit, Amount: 5000},
		},
	}

	_, _, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, input, fakeLedgerCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLedgerUnbalanced))
}

func TestFakeLedgerRepo_PostTransaction_UpdatesBalances(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	_, _, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)
	require.NoError(t, err)

	debitBal, err := repo.GetBalance(context.Background(), fakeLedgerDebitAcct)
	require.NoError(t, err)
	assert.Equal(t, int64(-10000), debitBal, "debit increases expense, balance goes negative")

	creditBal, err := repo.GetBalance(context.Background(), fakeLedgerCreditAcct)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), creditBal, "credit increases balance")
}

// ---------------------------------------------------------------------------
// GetBalance
// ---------------------------------------------------------------------------

func TestFakeLedgerRepo_GetBalance_ZeroForUnknownAccount(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	bal, err := repo.GetBalance(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)
}

func TestFakeLedgerRepo_GetBalance_AccumulatesMultipleTransactions(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	_, _, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)
	require.NoError(t, err)
	_, _, err = repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)
	require.NoError(t, err)

	bal, err := repo.GetBalance(context.Background(), fakeLedgerCreditAcct)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), bal, "two credits of 10000 = 20000")
}

// ---------------------------------------------------------------------------
// GetTransactionHistory
// ---------------------------------------------------------------------------

func TestFakeLedgerRepo_GetTransactionHistory_ReturnsEntriesForAccount(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	_, _, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)
	require.NoError(t, err)

	history, err := repo.GetTransactionHistory(context.Background(), fakeLedgerHouseholdID,
		ledger.HistoryQuery{AccountID: fakeLedgerDebitAcct, Offset: 0, Limit: 10})

	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "Test", history[0].Transaction.Description)
}

func TestFakeLedgerRepo_GetTransactionHistory_Pagination(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()

	for i := 0; i < 5; i++ {
		_, _, err := repo.PostTransaction(context.Background(), fakeLedgerHouseholdID, balancedInput(), fakeLedgerCallerID)
		require.NoError(t, err)
	}

	page, err := repo.GetTransactionHistory(context.Background(), fakeLedgerHouseholdID,
		ledger.HistoryQuery{AccountID: fakeLedgerDebitAcct, Offset: 1, Limit: 2})

	require.NoError(t, err)
	assert.Len(t, page, 2)
}

// ---------------------------------------------------------------------------
// SetError
// ---------------------------------------------------------------------------

func TestFakeLedgerRepo_SetError_InjectsError(t *testing.T) {
	repo := postgres.NewFakeLedgerRepository()
	injected := errors.New("injected")

	repo.SetError(injected)

	_, err := repo.GetBalance(context.Background(), uuid.New())
	assert.ErrorIs(t, err, injected)
}

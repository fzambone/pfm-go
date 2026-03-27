//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/types"
)

var ledgerFixedDate = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

// insertTestAccountForLedger creates a household + account and returns accountID.
func insertTestAccountForLedger(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	householdID := insertTestHousehold(t, pool)
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		"INSERT INTO accounts (household_id, name, account_type, currency_code) VALUES ($1, $2, $3, $4) RETURNING id",
		householdID, uuid.New().String(), "CHECKING", "USD",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertTwoAccountsSameHousehold creates a household with two accounts.
func insertTwoAccountsSameHousehold(t *testing.T, pool *pgxpool.Pool) (householdID, acct1, acct2 uuid.UUID) {
	t.Helper()
	householdID = insertTestHousehold(t, pool)
	err := pool.QueryRow(context.Background(),
		"INSERT INTO accounts (household_id, name, account_type, currency_code) VALUES ($1, $2, $3, $4) RETURNING id",
		householdID, "Acct1", "CHECKING", "USD",
	).Scan(&acct1)
	require.NoError(t, err)
	err = pool.QueryRow(context.Background(),
		"INSERT INTO accounts (household_id, name, account_type, currency_code) VALUES ($1, $2, $3, $4) RETURNING id",
		householdID, "Acct2", "SAVINGS", "USD",
	).Scan(&acct2)
	require.NoError(t, err)
	return
}

func balancedLedgerInput(debitAcct, creditAcct uuid.UUID) ledger.PostTransactionInput {
	return ledger.PostTransactionInput{
		Description:     "Test Transaction",
		TransactionDate: ledgerFixedDate,
		Entries: []ledger.EntryInput{
			{AccountID: debitAcct, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: creditAcct, EntryType: types.EntryTypeCredit, Amount: 10000},
		},
	}
}

// ---------------------------------------------------------------------------
// PostTransaction
// ---------------------------------------------------------------------------

func TestLedgerRepo_PostTransaction_StoresTransactionAndEntries(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	tx, entries, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tx.ID)
	assert.Equal(t, "Test Transaction", tx.Description)
	assert.Len(t, entries, 2)
	assert.False(t, tx.CreatedAt.IsZero())
}

func TestLedgerRepo_PostTransaction_UpdatesAccountBalances(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	_, _, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
	require.NoError(t, err)

	bal1, err := repo.GetBalance(ctx, acct1)
	require.NoError(t, err)
	assert.Equal(t, int64(-10000), bal1, "debit account balance decreases")

	bal2, err := repo.GetBalance(ctx, acct2)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), bal2, "credit account balance increases")
}

func TestLedgerRepo_PostTransaction_Atomic_RollbackOnFailure(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	tx := database.NewPostgresTransactor(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	simulatedErr := errors.New("simulated failure")

	err := tx.RunAtomic(ctx, func(txCtx context.Context) error {
		_, _, err := repo.PostTransaction(txCtx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
		if err != nil {
			return err
		}
		return simulatedErr
	})
	require.Error(t, err)

	// Balances should be unchanged after rollback.
	bal1, err := repo.GetBalance(ctx, acct1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal1, "balance unchanged after rollback")
}

func TestLedgerRepo_PostTransaction_UnbalancedFails(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	input := ledger.PostTransactionInput{
		Description:     "Unbalanced",
		TransactionDate: ledgerFixedDate,
		Entries: []ledger.EntryInput{
			{AccountID: acct1, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: acct2, EntryType: types.EntryTypeCredit, Amount: 5000},
		},
	}

	_, _, err := repo.PostTransaction(ctx, householdID, input, uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLedgerUnbalanced))
}

// ---------------------------------------------------------------------------
// GetBalance
// ---------------------------------------------------------------------------

func TestLedgerRepo_GetBalance_ZeroForNewAccount(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	acctID := insertTestAccountForLedger(t, pool)

	bal, err := repo.GetBalance(ctx, acctID)

	require.NoError(t, err)
	assert.Equal(t, int64(0), bal)
}

func TestLedgerRepo_GetBalance_AccumulatesMultipleTransactions(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	_, _, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
	require.NoError(t, err)
	_, _, err = repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
	require.NoError(t, err)

	bal, err := repo.GetBalance(ctx, acct2)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), bal)
}

// ---------------------------------------------------------------------------
// GetTransactionHistory
// ---------------------------------------------------------------------------

func TestLedgerRepo_GetTransactionHistory_ReturnsTransactionsForAccount(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	_, _, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
	require.NoError(t, err)

	history, err := repo.GetTransactionHistory(ctx, householdID,
		ledger.HistoryQuery{AccountID: acct1, Offset: 0, Limit: 10})

	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "Test Transaction", history[0].Transaction.Description)
	assert.Len(t, history[0].Entries, 2, "should include all entries for the transaction")
}

func TestLedgerRepo_GetTransactionHistory_Pagination(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	for i := 0; i < 5; i++ {
		_, _, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
		require.NoError(t, err)
	}

	page, err := repo.GetTransactionHistory(ctx, householdID,
		ledger.HistoryQuery{AccountID: acct1, Offset: 1, Limit: 2})

	require.NoError(t, err)
	assert.Len(t, page, 2)
}

func TestLedgerRepo_GetTransactionHistory_NoFilter_AllHouseholdTransactions(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewLedgerRepo(pool)
	householdID, acct1, acct2 := insertTwoAccountsSameHousehold(t, pool)

	_, _, err := repo.PostTransaction(ctx, householdID, balancedLedgerInput(acct1, acct2), uuid.Nil)
	require.NoError(t, err)

	history, err := repo.GetTransactionHistory(ctx, householdID,
		ledger.HistoryQuery{Offset: 0, Limit: 10})

	require.NoError(t, err)
	assert.Len(t, history, 1)
}

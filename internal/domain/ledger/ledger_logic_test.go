package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

func newLedgerLogic() (*ledger.LedgerLogic, *postgres.FakeLedgerRepository) {
	repo := postgres.NewFakeLedgerRepository()
	tx := database.NewFakeTransactor()
	clk := clock.NewFakeClock(fixedTime)
	logic := ledger.NewLedgerLogic(repo, tx, clk)
	return logic, repo
}

// ---------------------------------------------------------------------------
// PostTransaction
// ---------------------------------------------------------------------------

func TestLedgerLogic_PostTransaction_BalancedSucceeds(t *testing.T) {
	logic, _ := newLedgerLogic()

	tx, entries, err := logic.PostTransaction(context.Background(), testHouseholdID,
		postTransactionInputFactory(), testCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tx.ID)
	assert.Equal(t, "Test Transaction", tx.Description)
	assert.Len(t, entries, 2)
}

func TestLedgerLogic_PostTransaction_UnbalancedFails(t *testing.T) {
	logic, _ := newLedgerLogic()
	input := ledger.PostTransactionInput{
		Description:     "Unbalanced",
		TransactionDate: fixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: testDebitAcctID, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: testCreditAcctID, EntryType: types.EntryTypeCredit, Amount: 5000},
		},
	}

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLedgerUnbalanced))
}

func TestLedgerLogic_PostTransaction_ZeroEntries_FailsValidation(t *testing.T) {
	logic, _ := newLedgerLogic()
	input := ledger.PostTransactionInput{
		Description:     "Empty",
		TransactionDate: fixedTime,
		Entries:         []ledger.EntryInput{},
	}

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestLedgerLogic_PostTransaction_NonPositiveAmount_FailsValidation(t *testing.T) {
	logic, _ := newLedgerLogic()
	input := ledger.PostTransactionInput{
		Description:     "Bad Amount",
		TransactionDate: fixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: testDebitAcctID, EntryType: types.EntryTypeDebit, Amount: 0},
			{AccountID: testCreditAcctID, EntryType: types.EntryTypeCredit, Amount: 0},
		},
	}

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestLedgerLogic_PostTransaction_EmptyDescription_FailsValidation(t *testing.T) {
	logic, _ := newLedgerLogic()
	input := postTransactionInputFactory(func(i *ledger.PostTransactionInput) { i.Description = "" })

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestLedgerLogic_PostTransaction_SingleEntry_FailsBalance(t *testing.T) {
	logic, _ := newLedgerLogic()
	input := ledger.PostTransactionInput{
		Description:     "Single entry",
		TransactionDate: fixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: testDebitAcctID, EntryType: types.EntryTypeDebit, Amount: 10000},
		},
	}

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLedgerUnbalanced))
}

func TestLedgerLogic_PostTransaction_MultipleDebitsOneCredit_Succeeds(t *testing.T) {
	logic, _ := newLedgerLogic()
	thirdAcct := uuid.New()
	input := ledger.PostTransactionInput{
		Description:     "Split debit",
		TransactionDate: fixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: testDebitAcctID, EntryType: types.EntryTypeDebit, Amount: 3000},
			{AccountID: thirdAcct, EntryType: types.EntryTypeDebit, Amount: 7000},
			{AccountID: testCreditAcctID, EntryType: types.EntryTypeCredit, Amount: 10000},
		},
	}

	tx, entries, err := logic.PostTransaction(context.Background(), testHouseholdID, input, testCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, tx.ID)
	assert.Len(t, entries, 3)
}

// ---------------------------------------------------------------------------
// GetBalance
// ---------------------------------------------------------------------------

func TestLedgerLogic_GetBalance_ReturnsCorrectBalance(t *testing.T) {
	logic, _ := newLedgerLogic()

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID,
		postTransactionInputFactory(), testCallerID)
	require.NoError(t, err)

	bal, err := logic.GetBalance(context.Background(), testCreditAcctID)

	require.NoError(t, err)
	assert.Equal(t, int64(10000), bal)
}

func TestLedgerLogic_GetBalance_NegativeBalanceAllowed(t *testing.T) {
	logic, _ := newLedgerLogic()

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID,
		postTransactionInputFactory(), testCallerID)
	require.NoError(t, err)

	bal, err := logic.GetBalance(context.Background(), testDebitAcctID)

	require.NoError(t, err)
	assert.Equal(t, int64(-10000), bal, "debit account has negative balance")
}

// ---------------------------------------------------------------------------
// GetTransactionHistory
// ---------------------------------------------------------------------------

func TestLedgerLogic_GetTransactionHistory_ReturnsEntries(t *testing.T) {
	logic, _ := newLedgerLogic()

	_, _, err := logic.PostTransaction(context.Background(), testHouseholdID,
		postTransactionInputFactory(), testCallerID)
	require.NoError(t, err)

	history, err := logic.GetTransactionHistory(context.Background(), testHouseholdID,
		ledger.HistoryQuery{AccountID: testDebitAcctID, Offset: 0, Limit: 10})

	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "Test Transaction", history[0].Transaction.Description)
}

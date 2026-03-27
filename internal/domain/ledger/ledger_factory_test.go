package ledger_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/types"
)

var (
	fixedTime         = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testTransactionID = uuid.MustParse("00000000-0000-0000-0000-000000000040")
	testHouseholdID   = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	testDebitAcctID   = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	testCreditAcctID  = uuid.MustParse("00000000-0000-0000-0000-000000000021")
	testCallerID      = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testEntryID       = uuid.MustParse("00000000-0000-0000-0000-000000000050")
)

// transactionFactory returns a Transaction with all required fields set to non-zero defaults.
func transactionFactory(overrides ...func(*ledger.Transaction)) ledger.Transaction {
	tx := ledger.Transaction{
		ID:              testTransactionID,
		HouseholdID:     testHouseholdID,
		Description:     "Test Transaction",
		TransactionDate: fixedTime,
		CreatedAt:       fixedTime,
		CreatedBy:       testCallerID,
	}
	for _, o := range overrides {
		o(&tx)
	}
	return tx
}

// entryFactory returns an Entry with sensible defaults (DEBIT, positive amount).
func entryFactory(overrides ...func(*ledger.Entry)) ledger.Entry {
	e := ledger.Entry{
		ID:            testEntryID,
		TransactionID: testTransactionID,
		AccountID:     testDebitAcctID,
		EntryType:     types.EntryTypeDebit,
		Amount:        10000, // $100.00 in cents
		CreatedAt:     fixedTime,
	}
	for _, o := range overrides {
		o(&e)
	}
	return e
}

// postTransactionInputFactory returns a valid PostTransactionInput with two balanced
// entries (one debit, one credit of equal amount).
func postTransactionInputFactory(overrides ...func(*ledger.PostTransactionInput)) ledger.PostTransactionInput {
	input := ledger.PostTransactionInput{
		Description:     "Test Transaction",
		TransactionDate: fixedTime,
		Entries: []ledger.EntryInput{
			{AccountID: testDebitAcctID, EntryType: types.EntryTypeDebit, Amount: 10000},
			{AccountID: testCreditAcctID, EntryType: types.EntryTypeCredit, Amount: 10000},
		},
	}
	for _, o := range overrides {
		o(&input)
	}
	return input
}

// TestFactories_ProduceValidDefaults verifies that each factory returns fully
// populated values when called with no overrides.
func TestFactories_ProduceValidDefaults(t *testing.T) {
	t.Run("transactionFactory has non-zero fields", func(t *testing.T) {
		tx := transactionFactory()

		assert.NotEqual(t, uuid.Nil, tx.ID)
		assert.NotEqual(t, uuid.Nil, tx.HouseholdID)
		assert.NotEmpty(t, tx.Description)
		assert.False(t, tx.TransactionDate.IsZero())
		assert.False(t, tx.CreatedAt.IsZero())
		assert.NotEqual(t, uuid.Nil, tx.CreatedBy)
	})

	t.Run("transactionFactory override applies", func(t *testing.T) {
		tx := transactionFactory(func(tx *ledger.Transaction) { tx.Description = "Custom" })

		assert.Equal(t, "Custom", tx.Description)
	})

	t.Run("entryFactory defaults to DEBIT with positive amount", func(t *testing.T) {
		e := entryFactory()

		assert.NotEqual(t, uuid.Nil, e.ID)
		assert.NotEqual(t, uuid.Nil, e.TransactionID)
		assert.NotEqual(t, uuid.Nil, e.AccountID)
		assert.Equal(t, types.EntryTypeDebit, e.EntryType)
		assert.Greater(t, e.Amount, int64(0))
		assert.False(t, e.CreatedAt.IsZero())
	})

	t.Run("entryFactory override applies", func(t *testing.T) {
		e := entryFactory(func(e *ledger.Entry) { e.EntryType = types.EntryTypeCredit })

		assert.Equal(t, types.EntryTypeCredit, e.EntryType)
	})

	t.Run("postTransactionInputFactory has balanced entries", func(t *testing.T) {
		input := postTransactionInputFactory()

		assert.NotEmpty(t, input.Description)
		assert.False(t, input.TransactionDate.IsZero())
		assert.Len(t, input.Entries, 2, "default input should have exactly 2 entries")

		var debitSum, creditSum int64
		for _, e := range input.Entries {
			if e.EntryType == types.EntryTypeDebit {
				debitSum += e.Amount
			} else {
				creditSum += e.Amount
			}
		}
		assert.Equal(t, debitSum, creditSum, "default entries must balance")
	})

	t.Run("transactionFactory fixedTime is 2026-01-01", func(t *testing.T) {
		tx := transactionFactory()

		assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), tx.CreatedAt)
	})
}

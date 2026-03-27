package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/domain/ledger"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

// FakeLedgerRepository is a test double for domain/ledger.Repository.
// It stores transactions and entries in memory and maintains per-account balances.
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeLedgerRepository struct {
	mu           sync.RWMutex
	transactions []ledger.Transaction
	entries      []ledger.Entry
	balances     map[uuid.UUID]int64 // key: AccountID → balance
	err          error               // injected error returned by all methods
}

// NewFakeLedgerRepository returns an empty repository ready for use in tests.
func NewFakeLedgerRepository() *FakeLedgerRepository {
	return &FakeLedgerRepository{
		balances: make(map[uuid.UUID]int64),
	}
}

// SetError configures every subsequent method call to return err.
// Pass nil to clear the injected error.
func (f *FakeLedgerRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// PostTransaction validates that entries balance, stores the transaction and entries,
// and updates account balances. Credits increase balance, debits decrease it.
// Panics if called outside a test binary.
func (f *FakeLedgerRepository) PostTransaction(_ context.Context, householdID uuid.UUID, input ledger.PostTransactionInput, callerID uuid.UUID) (ledger.Transaction, []ledger.Entry, error) {
	if !testing.Testing() {
		panic("FakeLedgerRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return ledger.Transaction{}, nil, f.err
	}

	// Validate balance: sum of debits must equal sum of credits.
	var debitSum, creditSum int64
	for _, e := range input.Entries {
		switch e.EntryType {
		case types.EntryTypeDebit:
			debitSum += e.Amount
		case types.EntryTypeCredit:
			creditSum += e.Amount
		}
	}
	if debitSum != creditSum {
		return ledger.Transaction{}, nil, fmt.Errorf(message.ErrLedgerPostTransaction, message.ErrLedgerUnbalanced)
	}

	tx := ledger.Transaction{
		ID:              uuid.New(),
		HouseholdID:     householdID,
		Description:     input.Description,
		TransactionDate: input.TransactionDate,
		CreatedBy:       callerID,
	}
	f.transactions = append(f.transactions, tx)

	entries := make([]ledger.Entry, len(input.Entries))
	for i, ei := range input.Entries {
		entry := ledger.Entry{
			ID:            uuid.New(),
			TransactionID: tx.ID,
			AccountID:     ei.AccountID,
			EntryType:     ei.EntryType,
			Amount:        ei.Amount,
		}
		entries[i] = entry
		f.entries = append(f.entries, entry)

		// Update balance: credits increase, debits decrease.
		switch ei.EntryType {
		case types.EntryTypeCredit:
			f.balances[ei.AccountID] += ei.Amount
		case types.EntryTypeDebit:
			f.balances[ei.AccountID] -= ei.Amount
		}
	}

	return tx, entries, nil
}

// GetBalance returns the current balance for the given account.
// Returns 0 for accounts with no entries.
// Panics if called outside a test binary.
func (f *FakeLedgerRepository) GetBalance(_ context.Context, accountID uuid.UUID) (int64, error) {
	if !testing.Testing() {
		panic("FakeLedgerRepository: not for production use")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return 0, f.err
	}

	return f.balances[accountID], nil
}

// GetTransactionHistory returns transactions with their entries, filtered by account
// and paginated. Transactions are returned in insertion order.
// Panics if called outside a test binary.
func (f *FakeLedgerRepository) GetTransactionHistory(_ context.Context, householdID uuid.UUID, query ledger.HistoryQuery) ([]ledger.TransactionWithEntries, error) {
	if !testing.Testing() {
		panic("FakeLedgerRepository: not for production use")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return nil, f.err
	}

	// Collect transaction IDs that have entries matching the filter.
	txIDs := make(map[uuid.UUID]bool)
	for _, e := range f.entries {
		if query.AccountID != uuid.Nil && e.AccountID != query.AccountID {
			continue
		}
		txIDs[e.TransactionID] = true
	}

	// Build results in transaction order.
	var results []ledger.TransactionWithEntries
	for _, tx := range f.transactions {
		if tx.HouseholdID != householdID {
			continue
		}
		if !txIDs[tx.ID] {
			continue
		}
		var txEntries []ledger.Entry
		for _, e := range f.entries {
			if e.TransactionID == tx.ID {
				txEntries = append(txEntries, e)
			}
		}
		results = append(results, ledger.TransactionWithEntries{
			Transaction: tx,
			Entries:     txEntries,
		})
	}

	// Apply pagination.
	if query.Offset >= len(results) {
		return nil, nil
	}
	results = results[query.Offset:]
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

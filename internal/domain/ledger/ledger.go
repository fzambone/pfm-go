// Package ledger contains the business logic for double-entry financial transactions.
package ledger

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/types"
)

// Transaction represents a group of balanced ledger entries forming a single business event.
// Immutable: once created, a transaction is never updated or deleted.
type Transaction struct {
	ID              uuid.UUID
	HouseholdID     uuid.UUID
	Description     string
	TransactionDate time.Time // business date (date only, no time component)
	CreatedAt       time.Time
	CreatedBy       uuid.UUID
}

// Entry represents an individual debit or credit line within a transaction.
// Immutable: once created, an entry is never updated or deleted.
// Amount is always positive — EntryType determines the direction.
type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	EntryType     types.EntryType
	Amount        int64 // always positive; minor units (cents)
	CreatedAt     time.Time
}

// EntryInput carries the data for a single entry within a PostTransactionInput.
type EntryInput struct {
	AccountID uuid.UUID
	EntryType types.EntryType
	Amount    int64 // must be positive
}

// PostTransactionInput carries the caller-supplied data for posting a new transaction.
// HouseholdID and callerID come from context — they are not part of the input.
type PostTransactionInput struct {
	Description     string
	TransactionDate time.Time
	Entries         []EntryInput
}

// HistoryQuery carries pagination and filter parameters for transaction history.
type HistoryQuery struct {
	AccountID uuid.UUID // filter entries by account; uuid.Nil means no filter
	Offset    int
	Limit     int
}

// TransactionWithEntries pairs a transaction with its entries for history results.
type TransactionWithEntries struct {
	Transaction Transaction
	Entries     []Entry
}

// LedgerReader defines the read-only storage contract for the ledger domain.
type LedgerReader interface {
	// GetBalance returns the current balance for the given account.
	// Balance is computed as sum of credits minus sum of debits for the account.
	GetBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
	// GetTransactionHistory returns transactions with their entries, filtered and paginated.
	GetTransactionHistory(ctx context.Context, householdID uuid.UUID, query HistoryQuery) ([]TransactionWithEntries, error)
}

// LedgerWriter defines the write-only storage contract for the ledger domain.
type LedgerWriter interface {
	// PostTransaction creates a transaction with its entries and updates account balances
	// atomically. The caller is responsible for ensuring entries balance before calling.
	PostTransaction(ctx context.Context, householdID uuid.UUID, input PostTransactionInput, callerID uuid.UUID) (Transaction, []Entry, error)
}

// Repository defines the full storage contract for the ledger domain.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	LedgerReader
	LedgerWriter
}

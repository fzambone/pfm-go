// Package ledger contains the business logic for double-entry financial transactions.
package ledger

import (
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

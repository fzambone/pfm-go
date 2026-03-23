// Package account contains the business logic for financial account lifecycle management.
package account

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/types"
)

// Account represents a financial account belonging to a household.
// Balance is stored in minor units (cents/centavos) as int64 — never float.
type Account struct {
	ID           uuid.UUID
	HouseholdID  uuid.UUID
	Name         string
	AccountType  types.AccountType
	CurrencyCode types.CurrencyCode
	Balance      int64 // minor units (cents); matches BIGINT in DB
	Status       types.Status
	Version      int // optimistic concurrency version, mirrors the DB column
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
}

// CreateInput carries the caller-supplied data for creating a new account.
// HouseholdID and callerID come from context — they are not part of the input.
type CreateInput struct {
	Name         string
	AccountType  types.AccountType
	CurrencyCode types.CurrencyCode
}

// UpdateNameInput carries the new name for an account rename operation.
type UpdateNameInput struct {
	Name string
}

// UpdateBalanceInput carries the new balance for an account balance update.
// Balance is in minor units (cents/centavos).
type UpdateBalanceInput struct {
	Balance int64
}

// AccountReader defines the read-only storage contract for the account domain.
type AccountReader interface {
	// FindByID returns the active (non-deleted) account with the given ID.
	// Returns an error wrapping ErrAccountNotFound when no matching active account exists.
	FindByID(ctx context.Context, id uuid.UUID) (Account, error)
	// ListForHousehold returns all active accounts belonging to the given household.
	ListForHousehold(ctx context.Context, householdID uuid.UUID) ([]Account, error)
}

// AccountWriter defines the write-only storage contract for the account domain.
type AccountWriter interface {
	// Create persists a new account with zero balance and returns the saved entity
	// with server-assigned fields (ID, Version, timestamps).
	// Returns an error wrapping ErrAccountNameTaken when the name is already used
	// by another active account in the same household.
	Create(ctx context.Context, householdID uuid.UUID, input CreateInput, callerID uuid.UUID) (Account, error)
	// UpdateName changes the name of the account.
	// Returns an error wrapping ErrAccountVersionConflict when expectedVersion does not match.
	// Returns an error wrapping ErrAccountNameTaken when the new name conflicts.
	UpdateName(ctx context.Context, id uuid.UUID, input UpdateNameInput, expectedVersion int, callerID uuid.UUID) (Account, error)
	// UpdateBalance sets the account balance to the new value.
	// Returns an error wrapping ErrAccountVersionConflict when expectedVersion does not match.
	UpdateBalance(ctx context.Context, id uuid.UUID, input UpdateBalanceInput, expectedVersion int, callerID uuid.UUID) (Account, error)
	// Deactivate soft-deletes the account.
	// Idempotent — deactivating an already-deactivated account is not an error.
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// Repository defines the full storage contract for the account domain.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	AccountReader
	AccountWriter
}

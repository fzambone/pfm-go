// Package account contains the business logic for financial account lifecycle management.
package account

import (
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

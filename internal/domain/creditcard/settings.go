// Package creditcard contains the business logic for credit card settings management.
package creditcard

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Settings represents the credit-card-specific configuration for an account.
// Only accounts with AccountTypeCreditCard may have settings.
// One-to-one relationship with the accounts table (account_id is unique).
type Settings struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	ClosingDay  int   // billing cycle closing day (1-31)
	DueDay      int   // payment due day (1-31)
	LimitAmount int64 // credit limit in minor units (cents); matches BIGINT in DB
	Version     int   // optimistic concurrency version, mirrors the DB column
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
}

// CreateInput carries the caller-supplied data for creating credit card settings.
// AccountID and callerID come from context — they are not part of the input.
type CreateInput struct {
	ClosingDay  int
	DueDay      int
	LimitAmount int64
}

// UpdateClosingDayInput carries the new closing day for the billing cycle.
type UpdateClosingDayInput struct {
	ClosingDay int
}

// UpdateDueDayInput carries the new payment due day.
type UpdateDueDayInput struct {
	DueDay int
}

// UpdateLimitInput carries the new credit limit in minor units.
type UpdateLimitInput struct {
	LimitAmount int64
}

// SettingsReader defines the read-only storage contract for credit card settings.
type SettingsReader interface {
	// FindByAccountID returns the active settings for the given account.
	// Returns an error wrapping ErrCreditCardSettingsNotFound when no settings exist.
	FindByAccountID(ctx context.Context, accountID uuid.UUID) (Settings, error)
}

// SettingsWriter defines the write-only storage contract for credit card settings.
type SettingsWriter interface {
	// Create persists new credit card settings for the given account.
	// Returns an error wrapping ErrCreditCardSettingsExists when settings already exist.
	Create(ctx context.Context, accountID uuid.UUID, input CreateInput, callerID uuid.UUID) (Settings, error)
	// UpdateClosingDay changes the billing cycle closing day.
	// Returns an error wrapping ErrCreditCardSettingsVersionConflict when expectedVersion does not match.
	UpdateClosingDay(ctx context.Context, accountID uuid.UUID, input UpdateClosingDayInput, expectedVersion int, callerID uuid.UUID) (Settings, error)
	// UpdateDueDay changes the payment due day.
	// Returns an error wrapping ErrCreditCardSettingsVersionConflict when expectedVersion does not match.
	UpdateDueDay(ctx context.Context, accountID uuid.UUID, input UpdateDueDayInput, expectedVersion int, callerID uuid.UUID) (Settings, error)
	// UpdateLimit changes the credit limit.
	// Returns an error wrapping ErrCreditCardSettingsVersionConflict when expectedVersion does not match.
	UpdateLimit(ctx context.Context, accountID uuid.UUID, input UpdateLimitInput, expectedVersion int, callerID uuid.UUID) (Settings, error)
	// Delete soft-deletes the settings for the given account.
	// Idempotent — deleting already-deleted settings is not an error.
	Delete(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error
}

// Repository defines the full storage contract for credit card settings.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	SettingsReader
	SettingsWriter
}

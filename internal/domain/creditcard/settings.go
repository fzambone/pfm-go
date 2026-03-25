// Package creditcard contains the business logic for credit card settings management.
package creditcard

import (
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

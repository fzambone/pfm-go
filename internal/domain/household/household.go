// Package household contains the business logic for household lifecycle and membership management.
package household

import (
	"time"

	"github.com/google/uuid"
)

// Role represents the permission level of a household member.
// Restricted to RoleAdmin and RoleMember — matches the CHECK constraint in the DB.
type Role string

const (
	// RoleAdmin grants full management capabilities within the household.
	RoleAdmin Role = "ADMIN"
	// RoleMember grants read and limited write access within the household.
	RoleMember Role = "MEMBER"
)

// Status represents the lifecycle state of a household.
// Restricted to StatusActive and StatusInactive — matches the CHECK constraint in the DB.
type Status string

const (
	// StatusActive indicates the household is operational.
	StatusActive Status = "ACTIVE"
	// StatusInactive indicates the household has been deactivated.
	StatusInactive Status = "INACTIVE"
)

// Household represents the central organizing unit for all financial data.
// All household-scoped entities (accounts, ledger entries) belong to exactly one household.
type Household struct {
	ID        uuid.UUID
	Name      string
	Status    Status
	Version   int // optimistic concurrency version, mirrors the DB column
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// Membership represents a user's participation in a household with a specific role.
// Uses joined_at/invited_by instead of the full audit set because it is a read-only join table.
type Membership struct {
	HouseholdID uuid.UUID
	UserID      uuid.UUID
	Role        Role
	InvitedBy   uuid.UUID
	JoinedAt    time.Time
}

// CreateInput carries the caller-supplied data for creating a new household.
// The creating user's ID comes from context — it is not part of the input.
type CreateInput struct {
	Name string
}

// UpdateNameInput carries the new name for a household rename operation.
type UpdateNameInput struct {
	Name string
}

// AddMemberInput carries the data needed to invite a new member to a household.
// The inviter's ID comes from context — it is not part of the input.
type AddMemberInput struct {
	UserID uuid.UUID
	Role   Role
}

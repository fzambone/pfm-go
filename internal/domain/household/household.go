// Package household contains the business logic for household lifecycle and membership management.
package household

import (
	"context"
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

// HouseholdReader defines the read-only storage contract for the household domain.
type HouseholdReader interface {
	// FindByID returns the active (non-deleted) household with the given ID.
	// Returns an error wrapping ErrHouseholdNotFound when no matching active household exists.
	FindByID(ctx context.Context, id uuid.UUID) (Household, error)
	// ListForUser returns all active households where the given user has an active membership.
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Household, error)
}

// HouseholdWriter defines the write-only storage contract for the household domain.
type HouseholdWriter interface {
	// Create persists a new household and its founding admin membership atomically.
	// The callerID becomes both the household's created_by and the admin member.
	// Returns the saved household with server-assigned fields (ID, Version, timestamps).
	Create(ctx context.Context, input CreateInput, callerID uuid.UUID) (Household, error)
	// AddMember adds a new membership to the household.
	// The callerID is recorded as invited_by on the membership.
	AddMember(ctx context.Context, householdID uuid.UUID, input AddMemberInput, callerID uuid.UUID) (Membership, error)
	// RemoveMember soft-deletes the membership for the given user in the household.
	// Idempotent — removing an already-removed or non-existent membership is not an error.
	RemoveMember(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, callerID uuid.UUID) error
	// UpdateName changes the name of the household.
	// Returns an error wrapping ErrHouseholdVersionConflict when expectedVersion does not match.
	UpdateName(ctx context.Context, id uuid.UUID, input UpdateNameInput, expectedVersion int, callerID uuid.UUID) (Household, error)
	// Deactivate soft-deletes the household.
	// Idempotent — deactivating an already-deactivated household is not an error.
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// Repository defines the full storage contract for the household domain.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	HouseholdReader
	HouseholdWriter
}

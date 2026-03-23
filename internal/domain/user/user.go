// Package user contains the business logic for user authentication and lifecycle management.
package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated principal stored in the system.
type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string
	Version      int // optimistic concurrency version, mirrors the DB column
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
}

// RegisterInput carries the caller-supplied data for creating a new user.
// Validation is performed by the domain logic layer before this reaches the repository.
type RegisterInput struct {
	Email       string
	DisplayName string
	Password    string
}

// UpdateProfileInput carries the caller-supplied data for updating a user's profile.
type UpdateProfileInput struct {
	DisplayName string
}

// ChangePasswordInput carries the old and new passwords for a password change operation.
type ChangePasswordInput struct {
	OldPassword string
	NewPassword string
}

// LoginResult is returned on successful authentication.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
}

// UserReader defines the read-only storage contract for the user domain.
type UserReader interface {
	// FindByID returns the active (non-deleted) user with the given ID.
	// Returns an error wrapping ErrUserNotFound when no matching active user exists.
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	// FindByEmail returns the active (non-deleted) user with the given email.
	// The lookup is case-insensitive. Returns an error wrapping ErrLoginInvalidCredentials
	// when no matching active user exists.
	FindByEmail(ctx context.Context, email string) (User, error)
}

// UserWriter defines the write-only storage contract for the user domain.
type UserWriter interface {
	// Create persists a new user and returns the saved entity with server-assigned fields
	// (ID, Version, timestamps).
	Create(ctx context.Context, input RegisterInput, passwordHash string, callerID uuid.UUID) (User, error)
	// UpdateProfile changes the display name of an existing user.
	// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
	UpdateProfile(ctx context.Context, id uuid.UUID, input UpdateProfileInput, expectedVersion int, callerID uuid.UUID) (User, error)
	// ChangePassword replaces the password hash of an existing user.
	// Returns an error wrapping ErrUserVersionConflict when expectedVersion does not match.
	ChangePassword(ctx context.Context, id uuid.UUID, newHash string, expectedVersion int, callerID uuid.UUID) (User, error)
	// Deactivate soft-deletes the user. Idempotent — calling with an already-deactivated
	// or non-existent ID is not an error.
	Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error
}

// Repository defines the full storage contract for the user domain.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	UserReader
	UserWriter
}

// passwordVerifier abstracts the credential verification step.
// Structurally satisfied by port/auth.PasswordHasher.
type passwordVerifier interface {
	Verify(ctx context.Context, password, hash string) (bool, error)
}

// tokenIssuer abstracts token creation.
// Structurally satisfied by port/auth.TokenService.
type tokenIssuer interface {
	Issue(ctx context.Context, userID uuid.UUID, expiresIn time.Duration) (string, error)
}

// clocker abstracts the current time.
// Structurally satisfied by platform/clock.Clock.
type clocker interface {
	Now() time.Time
}

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

// Repository defines the storage contract required by LoginLogic.
// Defined at the consumer (domain) rather than the provider (adapter) per interface segregation.
type Repository interface {
	// FindByEmail returns the active (non-deleted) user with the given email.
	// The lookup is case-insensitive. Returns an error wrapping ErrLoginInvalidCredentials
	// when no matching active user exists.
	FindByEmail(ctx context.Context, email string) (User, error)
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

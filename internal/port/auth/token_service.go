package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TokenService issues and validates stateless authentication tokens.
// Tokens are self-contained: they carry the user ID and expiration so no
// server-side session storage is required.
type TokenService interface {
	// Issue creates a signed token for userID that expires after expiresIn.
	// The token includes issued-at, not-before, and expiration claims.
	Issue(ctx context.Context, userID uuid.UUID, expiresIn time.Duration) (string, error)

	// Validate verifies the token signature and expiration, returning the
	// embedded userID on success. Returns ErrTokenExpired or ErrTokenInvalid
	// for the respective failure modes.
	Validate(ctx context.Context, token string) (uuid.UUID, error)
}

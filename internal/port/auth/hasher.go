package auth

import "context"

// PasswordHasher hashes and verifies user passwords.
// Implementations must use constant-time comparison to prevent timing attacks.
type PasswordHasher interface {
	// Hash returns a cryptographic hash of password, including algorithm parameters
	// so the hash can be verified and migrated in the future.
	// Returns an error if password is empty or exceeds the maximum allowed length.
	Hash(ctx context.Context, password string) (string, error)

	// Verify reports whether password matches the stored hash.
	// Returns an error if the hash format is invalid or inputs fail validation.
	Verify(ctx context.Context, password, hash string) (bool, error)
}

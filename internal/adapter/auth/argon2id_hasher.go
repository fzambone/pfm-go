package auth

import (
	"context"
	"fmt"

	"github.com/alexedwards/argon2id"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/zambone/pfm-go/internal/message"
)

// Argon2idHasher implements port/auth.PasswordHasher using the Argon2id algorithm.
// The produced hash uses the PHC string format, which encodes the algorithm
// parameters alongside the salt and digest — enabling future parameter migration.
type Argon2idHasher struct {
	params *argon2id.Params
}

// NewArgon2idHasher creates an Argon2idHasher with the given params.
// Panics if params is nil to catch misconfigured wiring at startup.
func NewArgon2idHasher(params *argon2id.Params) *Argon2idHasher {
	if params == nil {
		panic("auth: NewArgon2idHasher requires non-nil params")
	}
	return &Argon2idHasher{params: params}
}

// DefaultArgon2idParams returns the library's recommended default parameters,
// suitable for most deployment environments. Tune Memory and Iterations for
// hardware-specific security/performance trade-offs.
func DefaultArgon2idParams() *argon2id.Params {
	return argon2id.DefaultParams
}

// Hash returns a PHC-encoded Argon2id hash of password.
// Each call produces a different hash because a new random salt is generated.
// Returns a validation error if password is empty or exceeds 1000 characters.
func (h *Argon2idHasher) Hash(ctx context.Context, password string) (string, error) {
	_, span := otel.Tracer("auth").Start(ctx, "Argon2idHasher.Hash")
	defer span.End()

	if err := validatePassword(password); err != nil {
		err = fmt.Errorf(message.ErrHasherHash, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	hash, err := argon2id.CreateHash(password, h.params)
	if err != nil {
		err = fmt.Errorf(message.ErrHasherHash, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	span.SetStatus(codes.Ok, "")
	return hash, nil
}

// Verify reports whether password matches the Argon2id hash.
// Uses constant-time comparison internally to prevent timing attacks.
func (h *Argon2idHasher) Verify(ctx context.Context, password, hash string) (bool, error) {
	_, span := otel.Tracer("auth").Start(ctx, "Argon2idHasher.Verify")
	defer span.End()

	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		err = fmt.Errorf(message.ErrHasherVerify, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetStatus(codes.Ok, "")
	return match, nil
}

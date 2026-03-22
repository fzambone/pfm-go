package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
)

// FakeHasher is a test double for port/auth.PasswordHasher.
// It uses a simple, predictable hash format ("fake:<password>") for deterministic tests.
//
// NOT SECURE — never use in production.
// FakeHasher panics if called outside a test binary (detected via testing.Testing()).
type FakeHasher struct{}

// NewFakeHasher returns a FakeHasher ready for use in tests.
func NewFakeHasher() *FakeHasher {
	return &FakeHasher{}
}

// Hash returns a predictable hash of password for use in tests.
// Panics if called outside a test binary. Returns a validation error if
// password is empty or exceeds 1000 characters.
func (f *FakeHasher) Hash(_ context.Context, password string) (string, error) {
	if !testing.Testing() {
		panic("FakeHasher: not for production use — wire Argon2idHasher instead")
	}
	if err := validatePassword(password); err != nil {
		return "", fmt.Errorf(message.ErrHasherHash, err)
	}
	return "fake:" + password, nil
}

// Verify reports whether password matches a hash previously produced by Hash.
// Panics if called outside a test binary.
func (f *FakeHasher) Verify(_ context.Context, password, hash string) (bool, error) {
	if !testing.Testing() {
		panic("FakeHasher: not for production use — wire Argon2idHasher instead")
	}
	return hash == "fake:"+password, nil
}

// validatePassword runs shared input validation used by both FakeHasher and Argon2idHasher.
func validatePassword(password string) error {
	r := validate.NewResult()
	r.Field("password", password,
		validate.Required,
		validate.MaxLen(1000),
	)
	return r.Error()
}

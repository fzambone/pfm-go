package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	authport "github.com/zambone/pfm-go/internal/port/auth"
)

// Compile-time assertion: FakeHasher must satisfy PasswordHasher.
var _ authport.PasswordHasher = (*authadapter.FakeHasher)(nil)

// TestFakeHasher_Hash_ReturnsDeterministicHash verifies that the same password
// always produces the same hash (deterministic for test repeatability).
func TestFakeHasher_Hash_ReturnsDeterministicHash(t *testing.T) {
	h := authadapter.NewFakeHasher()
	ctx := context.Background()

	hash1, err := h.Hash(ctx, "secret")
	require.NoError(t, err)

	hash2, err := h.Hash(ctx, "secret")
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "FakeHasher must produce identical hashes for the same password")
}

// TestFakeHasher_Verify_ReturnsTrueForCorrectPassword verifies that a password
// verifies successfully against its own hash.
func TestFakeHasher_Verify_ReturnsTrueForCorrectPassword(t *testing.T) {
	h := authadapter.NewFakeHasher()
	ctx := context.Background()

	hash, err := h.Hash(ctx, "secret")
	require.NoError(t, err)

	match, err := h.Verify(ctx, "secret", hash)
	require.NoError(t, err)
	assert.True(t, match)
}

// TestFakeHasher_Verify_ReturnsFalseForWrongPassword verifies that a different
// password does not match the hash.
func TestFakeHasher_Verify_ReturnsFalseForWrongPassword(t *testing.T) {
	h := authadapter.NewFakeHasher()
	ctx := context.Background()

	hash, err := h.Hash(ctx, "secret")
	require.NoError(t, err)

	match, err := h.Verify(ctx, "wrong", hash)
	require.NoError(t, err)
	assert.False(t, match)
}

// TestFakeHasher_Hash_InputValidation verifies that empty and over-length
// passwords are rejected, and that the boundary value (1000 chars) is accepted.
func TestFakeHasher_Hash_InputValidation(t *testing.T) {
	h := authadapter.NewFakeHasher()
	ctx := context.Background()

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty password", "", true},
		{"exceeds 1000 chars", strings.Repeat("x", 1001), true},
		{"exactly 1000 chars", strings.Repeat("x", 1000), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.Hash(ctx, tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

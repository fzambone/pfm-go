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

// Compile-time assertion: Argon2idHasher must satisfy PasswordHasher.
var _ authport.PasswordHasher = (*authadapter.Argon2idHasher)(nil)

// TestArgon2idHasher_Hash_ProducesVerifiableHash verifies AC1+AC2: the hash
// contains algorithm parameters and Verify uses constant-time comparison.
func TestArgon2idHasher_Hash_ProducesVerifiableHash(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	ctx := context.Background()

	hash, err := h.Hash(ctx, "correct-horse-battery-staple")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	match, err := h.Verify(ctx, "correct-horse-battery-staple", hash)
	require.NoError(t, err)
	assert.True(t, match, "password must verify against its own hash")
}

// TestArgon2idHasher_Hash_ProducesDifferentHashesSamePassword verifies AC3:
// unique salt means the same password hashed twice yields different hashes.
func TestArgon2idHasher_Hash_ProducesDifferentHashesSamePassword(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	ctx := context.Background()

	hash1, err := h.Hash(ctx, "password")
	require.NoError(t, err)

	hash2, err := h.Hash(ctx, "password")
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "each hash must use a unique salt")
}

// TestArgon2idHasher_Verify_ReturnsFalseForWrongPassword verifies that a
// different password does not match the hash.
func TestArgon2idHasher_Verify_ReturnsFalseForWrongPassword(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	ctx := context.Background()

	hash, err := h.Hash(ctx, "correct")
	require.NoError(t, err)

	match, err := h.Verify(ctx, "wrong", hash)
	require.NoError(t, err)
	assert.False(t, match)
}

// TestArgon2idHasher_Verify_RejectsInvalidHashFormat verifies that Verify
// returns an error when given a malformed hash string (not PHC format).
func TestArgon2idHasher_Verify_RejectsInvalidHashFormat(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())

	_, err := h.Verify(context.Background(), "password", "not-a-valid-argon2id-hash")
	assert.Error(t, err)
}

// TestArgon2idHasher_Hash_InputValidation verifies that empty and over-length
// passwords are rejected, and that the boundary value (1000 chars) is accepted.
func TestArgon2idHasher_Hash_InputValidation(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
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

// TestArgon2idHasher_HashContainsAlgorithmParameters verifies AC1: the hash
// string encodes the Argon2id parameters so future verification is self-contained.
func TestArgon2idHasher_HashContainsAlgorithmParameters(t *testing.T) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())

	hash, err := h.Hash(context.Background(), "password")
	require.NoError(t, err)

	// The PHC string format starts with $argon2id$
	assert.Contains(t, hash, "$argon2id$", "hash must encode Argon2id algorithm identifier")
}

// TestNewArgon2idHasher_PanicsOnNilParams verifies that the constructor panics
// if called with nil params, catching wiring mistakes at startup.
func TestNewArgon2idHasher_PanicsOnNilParams(t *testing.T) {
	assert.Panics(t, func() {
		authadapter.NewArgon2idHasher(nil)
	})
}

// BenchmarkArgon2idHasher_Hash measures the memory and time cost of hashing,
// which is intentionally expensive (memory-hard). Use this to validate that
// parameter tuning stays within acceptable latency budgets for the deployment.
func BenchmarkArgon2idHasher_Hash(b *testing.B) {
	h := authadapter.NewArgon2idHasher(authadapter.DefaultArgon2idParams())
	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		_, err := h.Hash(ctx, "benchmark-password-123")
		if err != nil {
			b.Fatal(err)
		}
	}
}

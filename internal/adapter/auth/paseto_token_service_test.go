package auth_test

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	authport "github.com/zambone/pfm-go/internal/port/auth"
)

// Compile-time assertion: PasetoTokenService must satisfy TokenService.
var _ authport.TokenService = (*authadapter.PasetoTokenService)(nil)

// validTestKeyHex is a 32-byte key expressed as 64 hex chars for test use.
var validTestKeyHex = hex.EncodeToString([]byte(strings.Repeat("k", 32)))

// TestPasetoTokenService_Issue_ThenValidate_ReturnsUserID verifies AC1+AC2:
// issued token encodes the user ID, and Validate recovers it.
func TestPasetoTokenService_Issue_ThenValidate_ReturnsUserID(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)
	ctx := context.Background()

	token, err := svc.Issue(ctx, testUserID, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	got, err := svc.Validate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, testUserID, got)
}

// TestPasetoTokenService_Issue_DifferentSaltsEachCall verifies AC (unique tokens):
// same user issued twice gets different tokens (random nonce per encrypt call).
func TestPasetoTokenService_Issue_DifferentSaltsEachCall(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)
	ctx := context.Background()

	t1, err := svc.Issue(ctx, testUserID, time.Hour)
	require.NoError(t, err)

	t2, err := svc.Issue(ctx, testUserID, time.Hour)
	require.NoError(t, err)

	assert.NotEqual(t, t1, t2, "each Issue must produce a unique token due to random nonce")
}

// TestPasetoTokenService_Validate_ExpiredToken_ReturnsErrTokenExpired verifies AC3.
func TestPasetoTokenService_Validate_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)
	ctx := context.Background()

	token, err := svc.Issue(ctx, testUserID, time.Hour)
	require.NoError(t, err)

	// Advance clock past expiry.
	clk.Advance(2 * time.Hour)

	_, err = svc.Validate(ctx, token)
	assert.ErrorIs(t, err, message.ErrTokenExpired)
}

// TestPasetoTokenService_Validate_TamperedToken_ReturnsErrTokenInvalid verifies AC4.
func TestPasetoTokenService_Validate_TamperedToken_ReturnsErrTokenInvalid(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = svc.Validate(ctx, "v4.local.tampered-garbage")
	assert.ErrorIs(t, err, message.ErrTokenInvalid)
}

// TestPasetoTokenService_Validate_MalformedToken_ReturnsErrTokenInvalid verifies
// the edge case: token format is checked before crypto verification.
func TestPasetoTokenService_Validate_MalformedToken_ReturnsErrTokenInvalid(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)

	_, err = svc.Validate(context.Background(), "not-a-token-at-all")
	assert.ErrorIs(t, err, message.ErrTokenInvalid)
}

// TestNewPasetoTokenService_RejectsEmptyKey verifies the edge case:
// an empty key is rejected at construction time.
func TestNewPasetoTokenService_RejectsEmptyKey(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	_, err := authadapter.NewPasetoTokenService("", clk)
	assert.Error(t, err)
}

// TestNewPasetoTokenService_RejectsShortKey verifies that a key shorter than
// 32 bytes (64 hex chars) is rejected.
func TestNewPasetoTokenService_RejectsShortKey(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	shortKey := hex.EncodeToString([]byte(strings.Repeat("k", 16))) // 16 bytes
	_, err := authadapter.NewPasetoTokenService(shortKey, clk)
	assert.Error(t, err)
}

// TestNewPasetoTokenService_RejectsNilClock verifies that nil clock panics.
func TestNewPasetoTokenService_RejectsNilClock(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = authadapter.NewPasetoTokenService(validTestKeyHex, nil)
	})
}

// TestPasetoTokenService_Validate_WrongKey_ReturnsErrTokenInvalid verifies that
// a token issued with one key cannot be validated with a different key.
func TestPasetoTokenService_Validate_WrongKey_ReturnsErrTokenInvalid(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc1, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	require.NoError(t, err)

	otherKey := hex.EncodeToString([]byte(strings.Repeat("z", 32)))
	svc2, err := authadapter.NewPasetoTokenService(otherKey, clk)
	require.NoError(t, err)

	token, err := svc1.Issue(context.Background(), testUserID, time.Hour)
	require.NoError(t, err)

	_, err = svc2.Validate(context.Background(), token)
	assert.ErrorIs(t, err, message.ErrTokenInvalid)
}

// BenchmarkPasetoTokenService_Validate measures the cost of decrypting and
// validating a token — called on every authenticated request.
func BenchmarkPasetoTokenService_Validate(b *testing.B) {
	clk := clock.NewFakeClock(fixedTime)
	svc, err := authadapter.NewPasetoTokenService(validTestKeyHex, clk)
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	token, err := svc.Issue(ctx, testUserID, time.Hour)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = svc.Validate(ctx, token)
	}
}

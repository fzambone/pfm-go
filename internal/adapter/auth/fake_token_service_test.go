package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authadapter "github.com/zambone/pfm-go/internal/adapter/auth"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	authport "github.com/zambone/pfm-go/internal/port/auth"
)

// Compile-time assertion: FakeTokenService must satisfy TokenService.
var _ authport.TokenService = (*authadapter.FakeTokenService)(nil)

var (
	fixedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

// TestFakeTokenService_Issue_ThenValidate_ReturnsUserID verifies AC1+AC2+AC6:
// issued token can be validated and returns the original user ID.
func TestFakeTokenService_Issue_ThenValidate_ReturnsUserID(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	token, err := svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	got, err := svc.Validate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, testUserID, got)
}

// TestFakeTokenService_Validate_ExpiredToken_ReturnsErrTokenExpired verifies AC3
// and the edge case: FakeTokenService supports expired tokens for testing.
func TestFakeTokenService_Validate_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	token, err := svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	// Advance clock past expiry.
	clk.Advance(2 * time.Hour)

	_, err = svc.Validate(ctx, token)
	assert.ErrorIs(t, err, message.ErrTokenExpired)
}

// TestFakeTokenService_Validate_UnknownToken_ReturnsErrTokenInvalid verifies AC4:
// a token that was never issued is rejected.
func TestFakeTokenService_Validate_UnknownToken_ReturnsErrTokenInvalid(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	_, err := svc.Validate(ctx, "not-a-real-token")
	assert.ErrorIs(t, err, message.ErrTokenInvalid)
}

// TestFakeTokenService_Issue_DifferentUsersProduceDifferentTokens verifies that
// each Issue call produces a unique token, preventing token aliasing.
func TestFakeTokenService_Issue_DifferentUsersProduceDifferentTokens(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	userA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userB := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tokenA, err := svc.Issue(ctx, userA, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	tokenB, err := svc.Issue(ctx, userB, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	assert.NotEqual(t, tokenA, tokenB)

	gotA, err := svc.Validate(ctx, tokenA)
	require.NoError(t, err)
	assert.Equal(t, userA, gotA)

	gotB, err := svc.Validate(ctx, tokenB)
	require.NoError(t, err)
	assert.Equal(t, userB, gotB)
}

// TestFakeTokenService_Issue_SameUserProducesUniqueTokens verifies that
// issuing twice for the same user yields different tokens (no aliasing).
func TestFakeTokenService_Issue_SameUserProducesUniqueTokens(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	t1, err := svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	t2, err := svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	assert.NotEqual(t, t1, t2, "each Issue call must produce a unique token")
}

// TestNewFakeTokenService_PanicsOnNilClock verifies that constructing a
// FakeTokenService with a nil clock panics at startup.
func TestNewFakeTokenService_PanicsOnNilClock(t *testing.T) {
	assert.Panics(t, func() {
		authadapter.NewFakeTokenService(nil)
	})
}

// TestFakeTokenService_Validate_TokenNotRenewable verifies the edge case:
// validating after expiry always returns ErrTokenExpired even if re-issued.
func TestFakeTokenService_Validate_TokenNotRenewable(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	svc := authadapter.NewFakeTokenService(clk)
	ctx := context.Background()

	oldToken, err := svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	clk.Advance(2 * time.Hour)

	// Issue a new token (which is valid); old token must still be expired.
	_, err = svc.Issue(ctx, testUserID, fixedTime.Add(time.Hour))
	require.NoError(t, err)

	_, err = svc.Validate(ctx, oldToken)
	assert.True(t, errors.Is(err, message.ErrTokenExpired), "expired token must stay expired")
}

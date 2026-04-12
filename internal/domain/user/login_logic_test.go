package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/auth"
	"github.com/zambone/pfm-go/internal/adapter/postgres"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
)

var (
	fixedTime  = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

const (
	testEmail    = "user@example.com"
	testPassword = "correct-horse-battery-staple"
)

// loginLogicFactory builds a LoginLogic pre-seeded with one active user.
// Returns the logic and the fake repo so individual tests can manipulate state.
func loginLogicFactory(t *testing.T) (*domainuser.LoginLogic, *postgres.FakeUserRepository) {
	t.Helper()
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()
	tokens := auth.NewFakeTokenService(clk)

	// Pre-hash the test password so the repo holds a realistic hash.
	hash, err := hasher.Hash(context.Background(), testPassword)
	require.NoError(t, err)

	repo.Add(domainuser.User{
		ID:           testUserID,
		Email:        testEmail,
		PasswordHash: hash,
	})

	logic := domainuser.NewLoginLogic(repo, hasher, tokens, clk, time.Hour)
	return logic, repo
}

// TestLoginLogic_ValidCredentials_ReturnsTokenAndExpiry verifies AC1:
// valid credentials produce a non-empty token and a future expiry time.
func TestLoginLogic_ValidCredentials_ReturnsTokenAndExpiry(t *testing.T) {
	logic, _ := loginLogicFactory(t)

	result, err := logic.Login(context.Background(), testEmail, testPassword)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, fixedTime.Add(time.Hour), result.ExpiresAt)
}

// TestLoginLogic_ValidCredentials_IsCaseInsensitive verifies that email lookup
// is case-insensitive: "USER@EXAMPLE.COM" finds the "user@example.com" record.
func TestLoginLogic_ValidCredentials_IsCaseInsensitive(t *testing.T) {
	logic, _ := loginLogicFactory(t)

	result, err := logic.Login(context.Background(), "USER@EXAMPLE.COM", testPassword)

	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
}

// TestLoginLogic_UserNotFound_ReturnsInvalidCredentials verifies AC3:
// a non-existent email produces the same generic error as a wrong password.
func TestLoginLogic_UserNotFound_ReturnsInvalidCredentials(t *testing.T) {
	logic, _ := loginLogicFactory(t)

	_, err := logic.Login(context.Background(), "nobody@example.com", testPassword)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"expected ErrLoginInvalidCredentials, got: %v", err)
}

// TestLoginLogic_WrongPassword_ReturnsInvalidCredentials verifies AC2 and AC5:
// wrong password produces the same generic error as a missing user.
func TestLoginLogic_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	logic, _ := loginLogicFactory(t)

	_, err := logic.Login(context.Background(), testEmail, "wrong-password")

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"expected ErrLoginInvalidCredentials, got: %v", err)
}

// TestLoginLogic_RepoInfraError_PropagatesError verifies that a non-auth repository
// error (e.g., DB down) is propagated to the caller, not swallowed.
func TestLoginLogic_RepoInfraError_PropagatesError(t *testing.T) {
	logic, repo := loginLogicFactory(t)
	infraErr := errors.New("connection refused")
	repo.SetError(infraErr)

	_, err := logic.Login(context.Background(), testEmail, testPassword)

	require.Error(t, err)
	assert.ErrorIs(t, err, infraErr)
	// Must NOT be treated as invalid credentials.
	assert.False(t, errors.Is(err, message.ErrLoginInvalidCredentials))
}

// FakeHasher and FakeTokenIssuer are minimal test doubles for error injection.
// They are unexported and scoped to this test file only.
type FakeHasher struct{ err error }

func (s *FakeHasher) Verify(_ context.Context, _, _ string) (bool, error) {
	return false, s.err
}

type FakeTokenIssuer struct{ err error }

func (s *FakeTokenIssuer) Issue(_ context.Context, _ uuid.UUID, _ time.Time) (string, error) {
	return "", s.err
}

// TestLoginLogic_HasherError_PropagatesError verifies that a hasher failure
// is propagated rather than silently treated as wrong credentials.
func TestLoginLogic_HasherError_PropagatesError(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	tokens := auth.NewFakeTokenService(clk)
	hasherErr := errors.New("argon2: internal error")
	hasher := &FakeHasher{err: hasherErr}

	hash := "fake:" + testPassword
	repo.Add(domainuser.User{ID: testUserID, Email: testEmail, PasswordHash: hash})
	logic := domainuser.NewLoginLogic(repo, hasher, tokens, clk, time.Hour)

	_, err := logic.Login(context.Background(), testEmail, testPassword)

	require.Error(t, err)
	assert.ErrorIs(t, err, hasherErr)
}

// TestLoginLogic_TokenIssueError_PropagatesError verifies that a token service
// failure is propagated to the caller.
func TestLoginLogic_TokenIssueError_PropagatesError(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()
	tokenErr := errors.New("paseto: key error")
	tokens := &FakeTokenIssuer{err: tokenErr}

	hash, err := hasher.Hash(context.Background(), testPassword)
	require.NoError(t, err)
	repo.Add(domainuser.User{ID: testUserID, Email: testEmail, PasswordHash: hash})
	logic := domainuser.NewLoginLogic(repo, hasher, tokens, clk, time.Hour)

	_, err = logic.Login(context.Background(), testEmail, testPassword)

	require.Error(t, err)
	assert.ErrorIs(t, err, tokenErr)
}

// TestNewLoginLogic_PanicsOnNilDependencies verifies that all nil guards fire
// at wiring time, not at request time.
func TestNewLoginLogic_PanicsOnNilDependencies(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()
	tokens := auth.NewFakeTokenService(clk)

	cases := []struct {
		name string
		fn   func()
	}{
		{"nil repo", func() { domainuser.NewLoginLogic(nil, hasher, tokens, clk, time.Hour) }},
		{"nil hasher", func() { domainuser.NewLoginLogic(repo, nil, tokens, clk, time.Hour) }},
		{"nil tokens", func() { domainuser.NewLoginLogic(repo, hasher, nil, clk, time.Hour) }},
		{"nil clk", func() { domainuser.NewLoginLogic(repo, hasher, tokens, nil, time.Hour) }},
		{"zero ttl", func() { domainuser.NewLoginLogic(repo, hasher, tokens, clk, 0) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, tc.fn)
		})
	}
}

// BenchmarkLoginLogic_ValidCredentials measures the per-request cost of the
// full login flow — hash verify + token issue — on the happy path.
func BenchmarkLoginLogic_ValidCredentials(b *testing.B) {
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()
	tokens := auth.NewFakeTokenService(clk)

	hash, err := hasher.Hash(context.Background(), testPassword)
	if err != nil {
		b.Fatal(err)
	}
	repo.Add(domainuser.User{
		ID:           testUserID,
		Email:        testEmail,
		PasswordHash: hash,
	})

	logic := domainuser.NewLoginLogic(repo, hasher, tokens, clk, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = logic.Login(context.Background(), testEmail, testPassword)
	}
}

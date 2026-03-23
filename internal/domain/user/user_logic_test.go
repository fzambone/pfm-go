package user_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/auth"
	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/validate"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// nonExistentID is a UUID guaranteed never to be seeded in any test repo.
var nonExistentID = uuid.MustParse("00000000-0000-0000-0000-000000000099")

// newUserLogic returns a UserLogic pre-seeded with one active user (testUserID, testEmail,
// password = "correct-horse-battery-staple" hashed with FakeHasher).
func newUserLogic(t *testing.T) (*domainuser.UserLogic, *postgres.FakeUserRepository) {
	t.Helper()
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()

	hash, err := hasher.Hash(context.Background(), testPassword)
	require.NoError(t, err)

	repo.Add(domainuser.User{
		ID:           testUserID,
		Email:        testEmail,
		PasswordHash: hash,
		Version:      1,
	})

	logic := domainuser.NewUserLogic(repo, hasher, clk)
	return logic, repo
}

// ---------------------------------------------------------------------------
// NewUserLogic nil guards
// ---------------------------------------------------------------------------

// TestNewUserLogic_PanicsOnNilDependencies verifies that all three nil guards
// fire at wiring time, not at request time.
func TestNewUserLogic_PanicsOnNilDependencies(t *testing.T) {
	clk := clock.NewFakeClock(fixedTime)
	repo := postgres.NewFakeUserRepository()
	hasher := auth.NewFakeHasher()

	cases := []struct {
		name string
		fn   func()
	}{
		{"nil repo", func() { domainuser.NewUserLogic(nil, hasher, clk) }},
		{"nil hasher", func() { domainuser.NewUserLogic(repo, nil, clk) }},
		{"nil clk", func() { domainuser.NewUserLogic(repo, hasher, nil) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, tc.fn)
		})
	}
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

// TestUserLogic_Register_HappyPath verifies that Register returns a User with
// a server-assigned ID, hashed password, and all input fields populated.
func TestUserLogic_Register_HappyPath(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := registerInputFactory(func(i *domainuser.RegisterInput) {
		i.Email = "new@example.com" // avoid collision with seed user
	})

	u, err := logic.Register(context.Background(), in, testUserID)

	require.NoError(t, err)
	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "new@example.com", u.Email)
	assert.Equal(t, in.DisplayName, u.DisplayName)
	assert.NotEqual(t, in.Password, u.PasswordHash, "password must be hashed, not stored in plain text")
	assert.Equal(t, 1, u.Version)
}

// TestUserLogic_Register_EmailAlreadyTaken verifies that registering with a
// duplicate email propagates ErrUserEmailTaken.
func TestUserLogic_Register_EmailAlreadyTaken(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := registerInputFactory() // default email = testEmail, already seeded

	_, err := logic.Register(context.Background(), in, testUserID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserEmailTaken),
		"expected ErrUserEmailTaken, got: %v", err)
}

// TestUserLogic_Register_ValidationErrors verifies that invalid inputs are
// rejected with a ValidationError before any repo call.
func TestUserLogic_Register_ValidationErrors(t *testing.T) {
	cases := []struct {
		name  string
		input domainuser.RegisterInput
		field string
	}{
		{
			name:  "empty email",
			input: registerInputFactory(func(i *domainuser.RegisterInput) { i.Email = "" }),
			field: "email",
		},
		{
			name:  "empty display name",
			input: registerInputFactory(func(i *domainuser.RegisterInput) { i.DisplayName = "" }),
			field: "display_name",
		},
		{
			name: "short password",
			input: registerInputFactory(func(i *domainuser.RegisterInput) {
				i.Email = "short@example.com"
				i.Password = "short"
			}),
			field: "password",
		},
	}

	logic, _ := newUserLogic(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := logic.Register(context.Background(), tc.input, testUserID)

			require.Error(t, err)
			var ve *validate.ValidationError
			require.True(t, errors.As(err, &ve),
				"expected ValidationError, got: %T %v", err, err)
			hasField := false
			for _, v := range ve.Violations {
				if v.Field == tc.field {
					hasField = true
					break
				}
			}
			assert.True(t, hasField, "expected violation on field %q, violations: %v", tc.field, ve.Violations)
		})
	}
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

// TestUserLogic_FindByID_HappyPath verifies that a seeded user is returned by ID.
func TestUserLogic_FindByID_HappyPath(t *testing.T) {
	logic, _ := newUserLogic(t)

	u, err := logic.FindByID(context.Background(), testUserID)

	require.NoError(t, err)
	assert.Equal(t, testUserID, u.ID)
}

// TestUserLogic_FindByID_NotFound verifies that ErrUserNotFound propagates from
// the repo when the ID does not exist.
func TestUserLogic_FindByID_NotFound(t *testing.T) {
	logic, _ := newUserLogic(t)

	_, err := logic.FindByID(context.Background(), nonExistentID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"expected ErrUserNotFound, got: %v", err)
}

// ---------------------------------------------------------------------------
// UpdateProfile
// ---------------------------------------------------------------------------

// TestUserLogic_UpdateProfile_HappyPath verifies that display name is updated
// and version increments.
func TestUserLogic_UpdateProfile_HappyPath(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := updateProfileInputFactory()

	u, err := logic.UpdateProfile(context.Background(), testUserID, in, 1, testUserID)

	require.NoError(t, err)
	assert.Equal(t, "Updated Name", u.DisplayName)
	assert.Equal(t, 2, u.Version)
}

// TestUserLogic_UpdateProfile_ValidationError verifies that an invalid display
// name is rejected before the repo is called.
func TestUserLogic_UpdateProfile_ValidationError(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := updateProfileInputFactory(func(i *domainuser.UpdateProfileInput) { i.DisplayName = "" })

	_, err := logic.UpdateProfile(context.Background(), testUserID, in, 1, testUserID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve), "expected ValidationError, got: %T %v", err, err)
}

// TestUserLogic_UpdateProfile_VersionConflict verifies that a stale version
// propagates ErrUserVersionConflict.
func TestUserLogic_UpdateProfile_VersionConflict(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := updateProfileInputFactory()

	_, err := logic.UpdateProfile(context.Background(), testUserID, in, 99, testUserID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

// TestUserLogic_ChangePassword_HappyPath verifies that the password hash is
// replaced and version increments.
func TestUserLogic_ChangePassword_HappyPath(t *testing.T) {
	logic, repo := newUserLogic(t)
	in := changePasswordInputFactory()

	u, err := logic.ChangePassword(context.Background(), testUserID, in, 1, testUserID)

	require.NoError(t, err)
	assert.Equal(t, 2, u.Version)

	// Verify new password is accepted after change.
	updated, err := repo.FindByID(context.Background(), testUserID)
	require.NoError(t, err)
	hasher := auth.NewFakeHasher()
	ok, err := hasher.Verify(context.Background(), in.NewPassword, updated.PasswordHash)
	require.NoError(t, err)
	assert.True(t, ok, "new password must be verifiable after change")
}

// TestUserLogic_ChangePassword_WrongOldPassword verifies that a wrong current
// password returns ErrLoginInvalidCredentials.
func TestUserLogic_ChangePassword_WrongOldPassword(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := changePasswordInputFactory(func(i *domainuser.ChangePasswordInput) {
		i.OldPassword = "wrong-password"
	})

	_, err := logic.ChangePassword(context.Background(), testUserID, in, 1, testUserID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"expected ErrLoginInvalidCredentials, got: %v", err)
}

// TestUserLogic_ChangePassword_VersionConflict verifies that a stale version
// propagates ErrUserVersionConflict.
func TestUserLogic_ChangePassword_VersionConflict(t *testing.T) {
	logic, _ := newUserLogic(t)
	in := changePasswordInputFactory()

	_, err := logic.ChangePassword(context.Background(), testUserID, in, 99, testUserID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

// TestUserLogic_Deactivate_HappyPath verifies that a deactivated user is no
// longer findable by ID.
func TestUserLogic_Deactivate_HappyPath(t *testing.T) {
	logic, _ := newUserLogic(t)

	err := logic.Deactivate(context.Background(), testUserID, testUserID)

	require.NoError(t, err)

	_, findErr := logic.FindByID(context.Background(), testUserID)
	assert.True(t, errors.Is(findErr, message.ErrUserNotFound),
		"deactivated user must not be findable")
}

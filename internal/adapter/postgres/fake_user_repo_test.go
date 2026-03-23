package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// fakeInput returns a RegisterInput with sensible defaults.
func fakeInput(overrides ...func(*domainuser.RegisterInput)) domainuser.RegisterInput {
	in := domainuser.RegisterInput{
		Email:       "user@example.com",
		DisplayName: "Test User",
		Password:    "correct-horse-battery-staple",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// callerID is a fixed UUID used as the actor for all fake tests.
var callerID = uuid.MustParse("00000000-0000-0000-0000-000000000099")

// TestFakeUserRepository_Create_StoresAndReturnsUser verifies the happy path:
// created user has server-assigned ID, correct fields, and Version = 1.
func TestFakeUserRepository_Create_StoresAndReturnsUser(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()

	u, err := repo.Create(ctx, fakeInput(), "hash123", callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, u.ID, "ID must be server-assigned")
	assert.Equal(t, "user@example.com", u.Email)
	assert.Equal(t, "Test User", u.DisplayName)
	assert.Equal(t, "hash123", u.PasswordHash)
	assert.Equal(t, 1, u.Version)
}

// TestFakeUserRepository_Create_FindableByID verifies the user is findable by ID after creation.
func TestFakeUserRepository_Create_FindableByID(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
}

// TestFakeUserRepository_Create_FindableByEmail verifies the user is findable by email after creation.
func TestFakeUserRepository_Create_FindableByEmail(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()

	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	found, err := repo.FindByEmail(ctx, created.Email)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

// TestFakeUserRepository_FindByID_NotFound verifies ErrUserNotFound is returned
// when the ID does not exist.
func TestFakeUserRepository_FindByID_NotFound(t *testing.T) {
	repo := postgres.NewFakeUserRepository()

	_, err := repo.FindByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"expected ErrUserNotFound, got: %v", err)
}

// TestFakeUserRepository_UpdateProfile_UpdatesDisplayName verifies the happy path:
// display name is updated and version increments.
func TestFakeUserRepository_UpdateProfile_UpdatesDisplayName(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	updated, err := repo.UpdateProfile(ctx, created.ID,
		domainuser.UpdateProfileInput{DisplayName: "New Name"},
		created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.DisplayName)
	assert.Equal(t, created.Version+1, updated.Version)
}

// TestFakeUserRepository_UpdateProfile_VersionConflict verifies that a stale version
// returns an error wrapping ErrUserVersionConflict.
func TestFakeUserRepository_UpdateProfile_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	_, err = repo.UpdateProfile(ctx, created.ID,
		domainuser.UpdateProfileInput{DisplayName: "New Name"},
		created.Version+99, callerID) // stale version

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// TestFakeUserRepository_UpdateProfile_ReflectedInBothIndexes verifies that
// an updated display name is visible when looking up by both email and ID.
func TestFakeUserRepository_UpdateProfile_ReflectedInBothIndexes(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	_, err = repo.UpdateProfile(ctx, created.ID,
		domainuser.UpdateProfileInput{DisplayName: "Updated"},
		created.Version, callerID)
	require.NoError(t, err)

	byID, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", byID.DisplayName)

	byEmail, err := repo.FindByEmail(ctx, created.Email)
	require.NoError(t, err)
	assert.Equal(t, "Updated", byEmail.DisplayName)
}

// TestFakeUserRepository_ChangePassword_UpdatesHash verifies the happy path:
// password hash is updated and version increments.
func TestFakeUserRepository_ChangePassword_UpdatesHash(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "old-hash", callerID)
	require.NoError(t, err)

	updated, err := repo.ChangePassword(ctx, created.ID, "new-hash", created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "new-hash", updated.PasswordHash)
	assert.Equal(t, created.Version+1, updated.Version)
}

// TestFakeUserRepository_ChangePassword_VersionConflict verifies that a stale version
// returns an error wrapping ErrUserVersionConflict.
func TestFakeUserRepository_ChangePassword_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "old-hash", callerID)
	require.NoError(t, err)

	_, err = repo.ChangePassword(ctx, created.ID, "new-hash", created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrUserVersionConflict),
		"expected ErrUserVersionConflict, got: %v", err)
}

// TestFakeUserRepository_Deactivate_RemovesFromBothIndexes verifies that a deactivated
// user is invisible to both FindByID and FindByEmail.
func TestFakeUserRepository_Deactivate_RemovesFromBothIndexes(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, callerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)
	assert.True(t, errors.Is(err, message.ErrUserNotFound),
		"deactivated user must not be findable by ID")

	_, err = repo.FindByEmail(ctx, created.Email)
	assert.True(t, errors.Is(err, message.ErrLoginInvalidCredentials),
		"deactivated user must not be findable by email")
}

// TestFakeUserRepository_Deactivate_IsIdempotent verifies that deactivating an
// already-deactivated (or never-existed) user does not return an error.
func TestFakeUserRepository_Deactivate_IsIdempotent(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	created, err := repo.Create(ctx, fakeInput(), "hash", callerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(ctx, created.ID, callerID))
	assert.NoError(t, repo.Deactivate(ctx, created.ID, callerID), "second deactivate must not error")
}

// TestFakeUserRepository_Add_DefaultsVersionToOne verifies that Add normalises
// a zero Version to 1 so callers don't need to remember to set it.
func TestFakeUserRepository_Add_DefaultsVersionToOne(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo.Add(domainuser.User{ID: id, Email: "a@b.com", Version: 0})

	u, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 1, u.Version)
}

// TestFakeUserRepository_Add_PreservesExplicitVersion verifies that a non-zero
// Version set by the caller is preserved unchanged.
func TestFakeUserRepository_Add_PreservesExplicitVersion(t *testing.T) {
	repo := postgres.NewFakeUserRepository()
	ctx := context.Background()
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	repo.Add(domainuser.User{ID: id, Email: "a@b.com", Version: 5})

	u, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 5, u.Version)
}

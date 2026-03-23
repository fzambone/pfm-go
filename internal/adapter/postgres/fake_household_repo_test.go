package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
)

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_Create_StoresHouseholdAndMembership(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	h, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, h.ID)
	assert.Equal(t, "Home", h.Name)
	assert.Equal(t, household.StatusActive, h.Status)
	assert.Equal(t, 1, h.Version)

	// The caller should be findable as a member via ListForUser.
	list, err := repo.ListForUser(context.Background(), callerID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, h.ID, list[0].ID)
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_FindByID_ReturnsHousehold(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	created, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Home", found.Name)
}

func TestFakeHouseholdRepository_FindByID_NotFound(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()

	_, err := repo.FindByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

// ---------------------------------------------------------------------------
// ListForUser
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_ListForUser_OnlyActiveMembers(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	_, err := repo.Create(context.Background(), household.CreateInput{Name: "H1"}, callerID)
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), household.CreateInput{Name: "H2"}, callerID)
	require.NoError(t, err)

	otherUser := uuid.New()
	list, err := repo.ListForUser(context.Background(), otherUser)

	require.NoError(t, err)
	assert.Empty(t, list, "user with no memberships should see no households")
}

// ---------------------------------------------------------------------------
// AddMember
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_AddMember_StoresAndReflectsInList(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()
	newMember := uuid.New()

	h, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	m, err := repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   household.RoleMember,
	}, callerID)

	require.NoError(t, err)
	assert.Equal(t, h.ID, m.HouseholdID)
	assert.Equal(t, newMember, m.UserID)
	assert.Equal(t, household.RoleMember, m.Role)

	list, err := repo.ListForUser(context.Background(), newMember)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestFakeHouseholdRepository_AddMember_DuplicateReturnsMemberExists(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()
	newMember := uuid.New()

	h, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   household.RoleMember,
	}, callerID)
	require.NoError(t, err)

	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: newMember,
		Role:   household.RoleMember,
	}, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberExists))
}

// ---------------------------------------------------------------------------
// RemoveMember
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_RemoveMember_SoftDeletes(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()
	member := uuid.New()

	h, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)
	_, err = repo.AddMember(context.Background(), h.ID, household.AddMemberInput{
		UserID: member,
		Role:   household.RoleMember,
	}, callerID)
	require.NoError(t, err)

	err = repo.RemoveMember(context.Background(), h.ID, member, callerID)
	require.NoError(t, err)

	list, err := repo.ListForUser(context.Background(), member)
	require.NoError(t, err)
	assert.Empty(t, list, "removed member should see no households")
}

func TestFakeHouseholdRepository_RemoveMember_IsIdempotent(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	h, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = repo.RemoveMember(context.Background(), h.ID, uuid.New(), callerID)
	assert.NoError(t, err, "removing a non-existent member should not error")
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_UpdateName_ChangesNameAndVersion(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	created, err := repo.Create(context.Background(), household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	updated, err := repo.UpdateName(context.Background(), created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestFakeHouseholdRepository_UpdateName_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	created, err := repo.Create(context.Background(), household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	_, err = repo.UpdateName(context.Background(), created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_Deactivate_SoftDeletes(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	created, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = repo.Deactivate(context.Background(), created.ID, callerID)
	require.NoError(t, err)

	_, err = repo.FindByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound),
		"deactivated household should not be findable")
}

func TestFakeHouseholdRepository_Deactivate_IsIdempotent(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	callerID := uuid.New()

	created, err := repo.Create(context.Background(), household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(context.Background(), created.ID, callerID))
	assert.NoError(t, repo.Deactivate(context.Background(), created.ID, callerID),
		"second deactivate must not error")
}

// ---------------------------------------------------------------------------
// SetError
// ---------------------------------------------------------------------------

func TestFakeHouseholdRepository_SetError_InjectsError(t *testing.T) {
	repo := postgres.NewFakeHouseholdRepository()
	injected := errors.New("injected")

	repo.SetError(injected)

	_, err := repo.FindByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, injected)
}

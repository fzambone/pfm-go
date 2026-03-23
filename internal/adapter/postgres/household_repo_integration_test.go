//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
)

// insertTestUser inserts a minimal user row via raw SQL and returns the server-assigned UUID.
// Needed because household_members has FK references to users(id).
func insertTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		"INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id",
		uuid.New().String()+"@test.com", integTestPasswordHash, "Test User",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestHouseholdRepo_Create_StoresHouseholdAndMembership(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, h.ID)
	assert.Equal(t, "Home", h.Name)
	assert.Equal(t, household.StatusActive, h.Status)
	assert.Equal(t, 1, h.Version)
	assert.False(t, h.CreatedAt.IsZero())

	members, err := repo.ListMembers(ctx, h.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, household.RoleAdmin, members[0].Role)
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestHouseholdRepo_FindByID_ReturnsHousehold(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Home", found.Name)
}

func TestHouseholdRepo_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)

	_, err := repo.FindByID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

func TestHouseholdRepo_FindByID_SoftDeleted_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, callerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

// ---------------------------------------------------------------------------
// ListForUser
// ---------------------------------------------------------------------------

func TestHouseholdRepo_ListForUser_ReturnsUserHouseholds(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	_, err := repo.Create(ctx, household.CreateInput{Name: "H1"}, callerID)
	require.NoError(t, err)
	_, err = repo.Create(ctx, household.CreateInput{Name: "H2"}, callerID)
	require.NoError(t, err)

	list, err := repo.ListForUser(ctx, callerID)

	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestHouseholdRepo_ListForUser_EmptyForUnknownUser(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	_, err := repo.Create(ctx, household.CreateInput{Name: "H1"}, callerID)
	require.NoError(t, err)

	otherUser := insertTestUser(t, pool)
	list, err := repo.ListForUser(ctx, otherUser)

	require.NoError(t, err)
	assert.Empty(t, list)
}

// ---------------------------------------------------------------------------
// FindMembership
// ---------------------------------------------------------------------------

func TestHouseholdRepo_FindMembership_ReturnsAdminMembership(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	m, err := repo.FindMembership(ctx, h.ID, callerID)

	require.NoError(t, err)
	assert.Equal(t, household.RoleAdmin, m.Role)
	assert.Equal(t, h.ID, m.HouseholdID)
}

func TestHouseholdRepo_FindMembership_NotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	otherUser := insertTestUser(t, pool)
	_, err = repo.FindMembership(ctx, h.ID, otherUser)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberNotFound))
}

// ---------------------------------------------------------------------------
// ListMembers
// ---------------------------------------------------------------------------

func TestHouseholdRepo_ListMembers_ReturnsAllActive(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	secondUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: secondUser,
		Role:   household.RoleMember,
	}, callerID)
	require.NoError(t, err)

	members, err := repo.ListMembers(ctx, h.ID)

	require.NoError(t, err)
	assert.Len(t, members, 2)
}

// ---------------------------------------------------------------------------
// AddMember
// ---------------------------------------------------------------------------

func TestHouseholdRepo_AddMember_StoresAndReturns(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	m, err := repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   household.RoleMember,
	}, callerID)

	require.NoError(t, err)
	assert.Equal(t, h.ID, m.HouseholdID)
	assert.Equal(t, newUser, m.UserID)
	assert.Equal(t, household.RoleMember, m.Role)
}

func TestHouseholdRepo_AddMember_DuplicateReturnsMemberExists(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   household.RoleMember,
	}, callerID)
	require.NoError(t, err)

	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   household.RoleMember,
	}, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberExists))
}

// ---------------------------------------------------------------------------
// RemoveMember
// ---------------------------------------------------------------------------

func TestHouseholdRepo_RemoveMember_SoftDeletes(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   household.RoleMember,
	}, callerID)
	require.NoError(t, err)

	err = repo.RemoveMember(ctx, h.ID, newUser, callerID)
	require.NoError(t, err)

	_, err = repo.FindMembership(ctx, h.ID, newUser)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberNotFound))
}

func TestHouseholdRepo_RemoveMember_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	otherUser := insertTestUser(t, pool)
	err = repo.RemoveMember(ctx, h.ID, otherUser, callerID)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestHouseholdRepo_UpdateName_ChangesNameAndVersion(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	updated, err := repo.UpdateName(ctx, created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestHouseholdRepo_UpdateName_VersionConflict(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Old"}, callerID)
	require.NoError(t, err)

	_, err = repo.UpdateName(ctx, created.ID,
		household.UpdateNameInput{Name: "New"}, created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestHouseholdRepo_Deactivate_SoftDeletes(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, callerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

func TestHouseholdRepo_Deactivate_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(ctx, created.ID, callerID))
	assert.NoError(t, repo.Deactivate(ctx, created.ID, callerID))
}

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
	"github.com/zambone/pfm-go/internal/platform/database"
	"github.com/zambone/pfm-go/internal/types"
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, h.ID)
	assert.Equal(t, "Home", h.Name)
	assert.Equal(t, types.StatusActive, h.Status)
	assert.Equal(t, 1, h.Version)
	assert.False(t, h.CreatedAt.IsZero())

	members, err := repo.ListMembers(ctx, h.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, types.RoleAdmin, members[0].Role)
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestHouseholdRepo_FindByID_ReturnsHousehold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)

	_, err := repo.FindByID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound))
}

func TestHouseholdRepo_FindByID_SoftDeleted_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	m, err := repo.FindMembership(ctx, h.ID, callerID)

	require.NoError(t, err)
	assert.Equal(t, types.RoleAdmin, m.Role)
	assert.Equal(t, h.ID, m.HouseholdID)
}

func TestHouseholdRepo_FindMembership_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	secondUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: secondUser,
		Role:   types.RoleMember,
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	m, err := repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   types.RoleMember,
	}, callerID)

	require.NoError(t, err)
	assert.Equal(t, h.ID, m.HouseholdID)
	assert.Equal(t, newUser, m.UserID)
	assert.Equal(t, types.RoleMember, m.Role)
}

func TestHouseholdRepo_AddMember_DuplicateReturnsMemberExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   types.RoleMember,
	}, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberExists))
}

// ---------------------------------------------------------------------------
// RemoveMember
// ---------------------------------------------------------------------------

func TestHouseholdRepo_RemoveMember_SoftDeletes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	newUser := insertTestUser(t, pool)
	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: newUser,
		Role:   types.RoleMember,
	}, callerID)
	require.NoError(t, err)

	err = repo.RemoveMember(ctx, h.ID, newUser, callerID)
	require.NoError(t, err)

	_, err = repo.FindMembership(ctx, h.ID, newUser)
	assert.True(t, errors.Is(err, message.ErrHouseholdMemberNotFound))
}

func TestHouseholdRepo_RemoveMember_IsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
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
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, callerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(ctx, created.ID, callerID))
	assert.NoError(t, repo.Deactivate(ctx, created.ID, callerID))
}

// ===========================================================================
// Cross-method and transactional integration tests (#68)
// ===========================================================================

// TestHouseholdRepo_Txn_CreateCommits verifies that when Create runs inside
// a committed transaction, both the household and its admin membership are
// visible in subsequent queries.
func TestHouseholdRepo_Txn_CreateCommits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)
	callerID := insertTestUser(t, pool)

	var householdID uuid.UUID
	err := tx.RunAtomic(ctx, func(txCtx context.Context) error {
		h, err := repo.Create(txCtx, household.CreateInput{Name: "Atomic Home"}, callerID)
		if err != nil {
			return err
		}
		householdID = h.ID
		return nil
	})
	require.NoError(t, err)

	// Both the household and membership should be visible after commit.
	found, err := repo.FindByID(ctx, householdID)
	require.NoError(t, err)
	assert.Equal(t, "Atomic Home", found.Name)

	members, err := repo.ListMembers(ctx, householdID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, types.RoleAdmin, members[0].Role)
}

// TestHouseholdRepo_Txn_CreateRollsBack verifies that when a transaction
// fails after Create, neither the household nor the membership is committed.
func TestHouseholdRepo_Txn_CreateRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	tx := database.NewPostgresTransactor(pool)
	callerID := insertTestUser(t, pool)

	var householdID uuid.UUID
	simulatedErr := errors.New("simulated failure after create")

	err := tx.RunAtomic(ctx, func(txCtx context.Context) error {
		h, err := repo.Create(txCtx, household.CreateInput{Name: "Doomed"}, callerID)
		if err != nil {
			return err
		}
		householdID = h.ID
		return simulatedErr // force rollback
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, simulatedErr)

	// Neither household nor membership should exist after rollback.
	_, err = repo.FindByID(ctx, householdID)
	assert.True(t, errors.Is(err, message.ErrHouseholdNotFound),
		"household must not exist after rollback, got: %v", err)

	list, err := repo.ListForUser(ctx, callerID)
	require.NoError(t, err)
	assert.Empty(t, list, "caller should have no households after rollback")
}

// TestHouseholdRepo_Workflow_DeactivateHidesFromAllMembers verifies that
// deactivating a household removes it from ListForUser for every member.
func TestHouseholdRepo_Workflow_DeactivateHidesFromAllMembers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	admin := insertTestUser(t, pool)
	member := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Shared"}, admin)
	require.NoError(t, err)

	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: member,
		Role:   types.RoleMember,
	}, admin)
	require.NoError(t, err)

	// Both users see the household before deactivation.
	adminList, err := repo.ListForUser(ctx, admin)
	require.NoError(t, err)
	assert.Len(t, adminList, 1)

	memberList, err := repo.ListForUser(ctx, member)
	require.NoError(t, err)
	assert.Len(t, memberList, 1)

	// Deactivate.
	err = repo.Deactivate(ctx, h.ID, admin)
	require.NoError(t, err)

	// Neither user sees the household after deactivation.
	adminList, err = repo.ListForUser(ctx, admin)
	require.NoError(t, err)
	assert.Empty(t, adminList, "admin should not see deactivated household")

	memberList, err = repo.ListForUser(ctx, member)
	require.NoError(t, err)
	assert.Empty(t, memberList, "member should not see deactivated household")
}

// TestHouseholdRepo_Workflow_AddRemoveMember verifies that adding a member
// makes the household visible in their list, and removing them hides it —
// while other members remain unaffected.
func TestHouseholdRepo_Workflow_AddRemoveMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	admin := insertTestUser(t, pool)
	member := insertTestUser(t, pool)

	h, err := repo.Create(ctx, household.CreateInput{Name: "Home"}, admin)
	require.NoError(t, err)

	_, err = repo.AddMember(ctx, h.ID, household.AddMemberInput{
		UserID: member,
		Role:   types.RoleMember,
	}, admin)
	require.NoError(t, err)

	// Member sees the household.
	memberList, err := repo.ListForUser(ctx, member)
	require.NoError(t, err)
	assert.Len(t, memberList, 1)

	// Remove the member.
	err = repo.RemoveMember(ctx, h.ID, member, admin)
	require.NoError(t, err)

	// Member no longer sees it.
	memberList, err = repo.ListForUser(ctx, member)
	require.NoError(t, err)
	assert.Empty(t, memberList, "removed member should not see household")

	// Admin still sees it.
	adminList, err := repo.ListForUser(ctx, admin)
	require.NoError(t, err)
	assert.Len(t, adminList, 1, "admin should still see household")
}

// TestHouseholdRepo_Workflow_VersionChain verifies optimistic concurrency:
// two sequential updates with correct version chain succeed (v1→v2→v3),
// but reusing the first version fails.
func TestHouseholdRepo_Workflow_VersionChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewHouseholdRepo(pool)
	callerID := insertTestUser(t, pool)

	created, err := repo.Create(ctx, household.CreateInput{Name: "V1"}, callerID)
	require.NoError(t, err)
	assert.Equal(t, 1, created.Version)

	v2, err := repo.UpdateName(ctx, created.ID,
		household.UpdateNameInput{Name: "V2"}, created.Version, callerID)
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)
	assert.Equal(t, "V2", v2.Name)

	v3, err := repo.UpdateName(ctx, created.ID,
		household.UpdateNameInput{Name: "V3"}, v2.Version, callerID)
	require.NoError(t, err)
	assert.Equal(t, 3, v3.Version)
	assert.Equal(t, "V3", v3.Name)

	// Reusing v1 should fail.
	_, err = repo.UpdateName(ctx, created.ID,
		household.UpdateNameInput{Name: "Stale"}, created.Version, callerID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrHouseholdVersionConflict))
}

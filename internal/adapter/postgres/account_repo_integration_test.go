//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/types"
)

var fixedAccountTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// insertTestHousehold inserts a minimal household row and returns its UUID.
// Needed because accounts have FK references to households(id).
func insertTestHousehold(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		"INSERT INTO households (name) VALUES ($1) RETURNING id",
		"Test Household",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func defaultAccountInput() account.CreateInput {
	return account.CreateInput{
		Name:         "Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestAccountRepo_Create_StoresAndReturns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	a, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.Equal(t, householdID, a.HouseholdID)
	assert.Equal(t, "Checking", a.Name)
	assert.Equal(t, types.AccountTypeChecking, a.AccountType)
	assert.Equal(t, types.CurrencyUSD, a.CurrencyCode)
	assert.Equal(t, int64(0), a.Balance)
	assert.Equal(t, types.StatusActive, a.Status)
	assert.Equal(t, 1, a.Version)
	assert.False(t, a.CreatedAt.IsZero())
}

func TestAccountRepo_Create_DuplicateName_ReturnsNameTaken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	_, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	_, err = repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNameTaken))
}

func TestAccountRepo_Create_CaseInsensitiveDuplicateName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	_, err := repo.Create(ctx, householdID, account.CreateInput{
		Name: "checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)

	_, err = repo.Create(ctx, householdID, account.CreateInput{
		Name: "CHECKING", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNameTaken))
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestAccountRepo_FindByID_ReturnsAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Checking", found.Name)
}

func TestAccountRepo_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)

	_, err := repo.FindByID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

func TestAccountRepo_FindByID_SoftDeleted_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, uuid.Nil)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

// ---------------------------------------------------------------------------
// ListForHousehold
// ---------------------------------------------------------------------------

func TestAccountRepo_ListForHousehold_ReturnsHouseholdAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	_, err := repo.Create(ctx, householdID, account.CreateInput{
		Name: "A1", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)
	_, err = repo.Create(ctx, householdID, account.CreateInput{
		Name: "A2", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)

	list, err := repo.ListForHousehold(ctx, householdID)

	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestAccountRepo_ListForHousehold_EmptyForOtherHousehold(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)
	otherHousehold := insertTestHousehold(t, pool)

	_, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	list, err := repo.ListForHousehold(ctx, otherHousehold)

	require.NoError(t, err)
	assert.Empty(t, list)
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestAccountRepo_UpdateName_ChangesNameAndVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	updated, err := repo.UpdateName(ctx, created.ID,
		account.UpdateNameInput{Name: "Savings"}, created.Version, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, "Savings", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestAccountRepo_UpdateName_VersionConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	_, err = repo.UpdateName(ctx, created.ID,
		account.UpdateNameInput{Name: "Savings"}, created.Version+99, uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// ---------------------------------------------------------------------------
// UpdateBalance
// ---------------------------------------------------------------------------

func TestAccountRepo_UpdateBalance_ChangesBalanceAndVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	updated, err := repo.UpdateBalance(ctx, created.ID,
		account.UpdateBalanceInput{Balance: 50000}, created.Version, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, int64(50000), updated.Balance)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestAccountRepo_UpdateBalance_VersionConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	_, err = repo.UpdateBalance(ctx, created.ID,
		account.UpdateBalanceInput{Balance: 50000}, created.Version+99, uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestAccountRepo_Deactivate_SoftDeletes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, uuid.Nil)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, created.ID)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

func TestAccountRepo_Deactivate_IsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(ctx, created.ID, uuid.Nil))
	assert.NoError(t, repo.Deactivate(ctx, created.ID, uuid.Nil))
}

// ===========================================================================
// Cross-method and workflow integration tests (#88)
// ===========================================================================

// TestAccountRepo_Workflow_DeactivateHidesFromList verifies that a deactivated
// account no longer appears in ListForHousehold results.
func TestAccountRepo_Workflow_DeactivateHidesFromList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	a1, err := repo.Create(ctx, householdID, account.CreateInput{
		Name: "Keep", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)

	a2, err := repo.Create(ctx, householdID, account.CreateInput{
		Name: "Remove", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, a2.ID, uuid.Nil)
	require.NoError(t, err)

	list, err := repo.ListForHousehold(ctx, householdID)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, a1.ID, list[0].ID)
}

// TestAccountRepo_Workflow_VersionChain verifies optimistic concurrency:
// two sequential updates with correct version chain succeed (v1→v2→v3),
// but reusing the first version fails.
func TestAccountRepo_Workflow_VersionChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 1, created.Version)

	v2, err := repo.UpdateName(ctx, created.ID,
		account.UpdateNameInput{Name: "V2"}, created.Version, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)

	v3, err := repo.UpdateName(ctx, created.ID,
		account.UpdateNameInput{Name: "V3"}, v2.Version, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 3, v3.Version)

	// Reusing v1 should fail.
	_, err = repo.UpdateName(ctx, created.ID,
		account.UpdateNameInput{Name: "Stale"}, created.Version, uuid.Nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// TestAccountRepo_Workflow_NonZeroBalanceDeactivateRejected verifies that the
// AccountLogic layer rejects deactivation when balance is non-zero, using
// the real postgres adapter.
func TestAccountRepo_Workflow_NonZeroBalanceDeactivateRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	clk := clock.NewFakeClock(fixedAccountTime)
	logic := account.NewAccountLogic(repo, clk)
	householdID := insertTestHousehold(t, pool)

	created, err := logic.Create(ctx, householdID, account.CreateInput{
		Name: "Funded", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, uuid.Nil)
	require.NoError(t, err)

	// Set a non-zero balance.
	_, err = repo.UpdateBalance(ctx, created.ID,
		account.UpdateBalanceInput{Balance: 50000}, created.Version, uuid.Nil)
	require.NoError(t, err)

	// Logic layer should reject deactivation.
	err = logic.Deactivate(ctx, created.ID, uuid.Nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountBalanceNotZero))

	// Account should still be findable.
	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Funded", found.Name)
}

// TestAccountRepo_Workflow_DeactivatedNameReusable verifies that after an account
// is soft-deleted, its name can be reused by a new account in the same household
// (the partial unique index only covers non-deleted rows).
func TestAccountRepo_Workflow_DeactivatedNameReusable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewAccountRepo(pool)
	householdID := insertTestHousehold(t, pool)

	created, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)

	err = repo.Deactivate(ctx, created.ID, uuid.Nil)
	require.NoError(t, err)

	// Same name should now succeed — the deactivated row has deleted_at set.
	reused, err := repo.Create(ctx, householdID, defaultAccountInput(), uuid.Nil)
	require.NoError(t, err)
	assert.NotEqual(t, created.ID, reused.ID)
	assert.Equal(t, created.Name, reused.Name)
}

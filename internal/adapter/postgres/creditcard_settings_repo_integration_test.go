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
	"github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
)

// insertTestAccount inserts a minimal credit card account and returns its UUID.
// Needs a household first (FK constraint).
func insertTestAccount(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	householdID := insertTestHousehold(t, pool)
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		"INSERT INTO accounts (household_id, name, account_type, currency_code) VALUES ($1, $2, $3, $4) RETURNING id",
		householdID, uuid.New().String(), "CREDIT_CARD", "USD",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func defaultCCInput() creditcard.CreateInput {
	return creditcard.CreateInput{ClosingDay: 25, DueDay: 10, LimitAmount: 500000}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_Create_StoresAndReturns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	s, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, accountID, s.AccountID)
	assert.Equal(t, 25, s.ClosingDay)
	assert.Equal(t, 10, s.DueDay)
	assert.Equal(t, int64(500000), s.LimitAmount)
	assert.Equal(t, 1, s.Version)
	assert.False(t, s.CreatedAt.IsZero())
}

func TestCCSettingsRepo_Create_Duplicate_ReturnsExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	_, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	_, err = repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsExists))
}

// ---------------------------------------------------------------------------
// FindByAccountID
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_FindByAccountID_ReturnsSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	created, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	found, err := repo.FindByAccountID(ctx, accountID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, 25, found.ClosingDay)
}

func TestCCSettingsRepo_FindByAccountID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)

	_, err := repo.FindByAccountID(ctx, uuid.MustParse("00000000-0000-0000-0000-000000000099"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

func TestCCSettingsRepo_FindByAccountID_SoftDeleted_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	_, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	err = repo.Delete(ctx, accountID, uuid.Nil)
	require.NoError(t, err)

	_, err = repo.FindByAccountID(ctx, accountID)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

// ---------------------------------------------------------------------------
// UpdateClosingDay
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_UpdateClosingDay_ChangesAndVersions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	created, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	updated, err := repo.UpdateClosingDay(ctx, accountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, created.Version, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, 15, updated.ClosingDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestCCSettingsRepo_UpdateClosingDay_VersionConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	_, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	_, err = repo.UpdateClosingDay(ctx, accountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, 99, uuid.Nil)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsVersionConflict))
}

// ---------------------------------------------------------------------------
// UpdateDueDay
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_UpdateDueDay_ChangesAndVersions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	created, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	updated, err := repo.UpdateDueDay(ctx, accountID,
		creditcard.UpdateDueDayInput{DueDay: 5}, created.Version, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, 5, updated.DueDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

// ---------------------------------------------------------------------------
// UpdateLimit
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_UpdateLimit_ChangesAndVersions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	created, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	updated, err := repo.UpdateLimit(ctx, accountID,
		creditcard.UpdateLimitInput{LimitAmount: 1000000}, created.Version, uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, int64(1000000), updated.LimitAmount)
	assert.Equal(t, created.Version+1, updated.Version)
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestCCSettingsRepo_Delete_SoftDeletes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	_, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	err = repo.Delete(ctx, accountID, uuid.Nil)
	require.NoError(t, err)

	_, err = repo.FindByAccountID(ctx, accountID)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

func TestCCSettingsRepo_Delete_IsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := sharedDB.NewPool(t, ctx)
	repo := postgres.NewCreditCardSettingsRepo(pool)
	accountID := insertTestAccount(t, pool)

	_, err := repo.Create(ctx, accountID, defaultCCInput(), uuid.Nil)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, accountID, uuid.Nil))
	assert.NoError(t, repo.Delete(ctx, accountID, uuid.Nil))
}

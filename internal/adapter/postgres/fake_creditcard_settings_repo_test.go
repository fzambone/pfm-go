package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
)

var fakeCCAccountID = uuid.MustParse("00000000-0000-0000-0000-000000000020")
var fakeCCCallerID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func defaultCCCreateInput() creditcard.CreateInput {
	return creditcard.CreateInput{ClosingDay: 25, DueDay: 10, LimitAmount: 500000}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_Create_StoresAndReturns(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	s, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, fakeCCAccountID, s.AccountID)
	assert.Equal(t, 25, s.ClosingDay)
	assert.Equal(t, 10, s.DueDay)
	assert.Equal(t, int64(500000), s.LimitAmount)
	assert.Equal(t, 1, s.Version)
}

func TestFakeCCSettingsRepo_Create_Duplicate_ReturnsExists(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	_, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsExists))
}

// ---------------------------------------------------------------------------
// FindByAccountID
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_FindByAccountID_ReturnsSettings(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	created, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	found, err := repo.FindByAccountID(context.Background(), fakeCCAccountID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestFakeCCSettingsRepo_FindByAccountID_NotFound(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	_, err := repo.FindByAccountID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

// ---------------------------------------------------------------------------
// UpdateClosingDay
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_UpdateClosingDay_ChangesAndVersions(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	created, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateClosingDay(context.Background(), fakeCCAccountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, created.Version, fakeCCCallerID)

	require.NoError(t, err)
	assert.Equal(t, 15, updated.ClosingDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestFakeCCSettingsRepo_UpdateClosingDay_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	created, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	_, err = repo.UpdateClosingDay(context.Background(), fakeCCAccountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, created.Version+99, fakeCCCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsVersionConflict))
}

// ---------------------------------------------------------------------------
// UpdateDueDay
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_UpdateDueDay_ChangesAndVersions(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	created, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateDueDay(context.Background(), fakeCCAccountID,
		creditcard.UpdateDueDayInput{DueDay: 5}, created.Version, fakeCCCallerID)

	require.NoError(t, err)
	assert.Equal(t, 5, updated.DueDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

// ---------------------------------------------------------------------------
// UpdateLimit
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_UpdateLimit_ChangesAndVersions(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	created, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateLimit(context.Background(), fakeCCAccountID,
		creditcard.UpdateLimitInput{LimitAmount: 1000000}, created.Version, fakeCCCallerID)

	require.NoError(t, err)
	assert.Equal(t, int64(1000000), updated.LimitAmount)
	assert.Equal(t, created.Version+1, updated.Version)
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_Delete_RemovesSettings(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	_, err := repo.Create(context.Background(), fakeCCAccountID, defaultCCCreateInput(), fakeCCCallerID)
	require.NoError(t, err)

	err = repo.Delete(context.Background(), fakeCCAccountID, fakeCCCallerID)
	require.NoError(t, err)

	_, err = repo.FindByAccountID(context.Background(), fakeCCAccountID)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

func TestFakeCCSettingsRepo_Delete_IsIdempotent(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()

	err := repo.Delete(context.Background(), uuid.New(), fakeCCCallerID)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// SetError
// ---------------------------------------------------------------------------

func TestFakeCCSettingsRepo_SetError_InjectsError(t *testing.T) {
	repo := postgres.NewFakeCreditCardSettingsRepository()
	injected := errors.New("injected")

	repo.SetError(injected)

	_, err := repo.FindByAccountID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, injected)
}

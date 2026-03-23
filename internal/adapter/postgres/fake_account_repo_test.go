package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zambone/pfm-go/internal/adapter/postgres"
	"github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

var (
	fakeAccountHouseholdID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	fakeAccountCallerID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_Create_StoresAndReturns(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	a, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name:         "Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.Equal(t, "Checking", a.Name)
	assert.Equal(t, types.AccountTypeChecking, a.AccountType)
	assert.Equal(t, int64(0), a.Balance)
	assert.Equal(t, 1, a.Version)
}

func TestFakeAccountRepository_Create_DuplicateName_ReturnsNameTaken(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	_, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name:         "Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name:         "Checking",
		AccountType:  types.AccountTypeSavings,
		CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNameTaken))
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_FindByID_ReturnsAccount(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestFakeAccountRepository_FindByID_NotFound(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	_, err := repo.FindByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

// ---------------------------------------------------------------------------
// ListForHousehold
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_ListForHousehold_ReturnsOnlyHouseholdAccounts(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()
	otherHousehold := uuid.New()

	_, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "A1", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), otherHousehold, account.CreateInput{
		Name: "A2", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	list, err := repo.ListForHousehold(context.Background(), fakeAccountHouseholdID)

	require.NoError(t, err)
	assert.Len(t, list, 1)
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_UpdateName_ChangesNameAndVersion(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Old", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: "New"}, created.Version, fakeAccountCallerID)

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestFakeAccountRepository_UpdateName_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Old", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	_, err = repo.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: "New"}, created.Version+99, fakeAccountCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

func TestFakeAccountRepository_UpdateName_DuplicateName_ReturnsNameTaken(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	_, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Existing", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)
	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Other", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	_, err = repo.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: "Existing"}, created.Version, fakeAccountCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNameTaken))
}

// ---------------------------------------------------------------------------
// UpdateBalance
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_UpdateBalance_ChangesBalanceAndVersion(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	updated, err := repo.UpdateBalance(context.Background(), created.ID,
		account.UpdateBalanceInput{Balance: 10000}, created.Version, fakeAccountCallerID)

	require.NoError(t, err)
	assert.Equal(t, int64(10000), updated.Balance)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestFakeAccountRepository_UpdateBalance_VersionConflict(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	_, err = repo.UpdateBalance(context.Background(), created.ID,
		account.UpdateBalanceInput{Balance: 10000}, created.Version+99, fakeAccountCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_Deactivate_SoftDeletes(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	err = repo.Deactivate(context.Background(), created.ID, fakeAccountCallerID)
	require.NoError(t, err)

	_, err = repo.FindByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

func TestFakeAccountRepository_Deactivate_IsIdempotent(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()

	created, err := repo.Create(context.Background(), fakeAccountHouseholdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, fakeAccountCallerID)
	require.NoError(t, err)

	require.NoError(t, repo.Deactivate(context.Background(), created.ID, fakeAccountCallerID))
	assert.NoError(t, repo.Deactivate(context.Background(), created.ID, fakeAccountCallerID))
}

// ---------------------------------------------------------------------------
// SetError
// ---------------------------------------------------------------------------

func TestFakeAccountRepository_SetError_InjectsError(t *testing.T) {
	repo := postgres.NewFakeAccountRepository()
	injected := errors.New("injected")

	repo.SetError(injected)

	_, err := repo.FindByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, injected)
}

package account_test

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
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

func newAccountLogic() (*account.AccountLogic, *postgres.FakeAccountRepository, uuid.UUID, uuid.UUID) {
	repo := postgres.NewFakeAccountRepository()
	clk := clock.NewFakeClock(fixedTime)
	logic := account.NewAccountLogic(repo, clk)
	return logic, repo, testHouseholdID, testCallerID
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestAccountLogic_Create_HappyPath(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	a, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name:         "Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}, callerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.Equal(t, "Checking", a.Name)
	assert.Equal(t, int64(0), a.Balance)
	assert.Equal(t, types.StatusActive, a.Status)
	assert.Equal(t, 1, a.Version)
}

func TestAccountLogic_Create_EmptyName_FailsValidation(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	_, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name:         "",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestAccountLogic_Create_InvalidAccountType_FailsValidation(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	_, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name:         "Bad Type",
		AccountType:  types.AccountType("UNKNOWN"),
		CurrencyCode: types.CurrencyUSD,
	}, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestAccountLogic_Create_InvalidCurrency_FailsValidation(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	_, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name:         "Bad Currency",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyCode("GBP"),
	}, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestAccountLogic_Create_DuplicateName_ReturnsError(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	_, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNameTaken))
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestAccountLogic_FindByID_HappyPath(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	found, err := logic.FindByID(context.Background(), created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestAccountLogic_FindByID_NotFound(t *testing.T) {
	logic, _, _, _ := newAccountLogic()

	_, err := logic.FindByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

// ---------------------------------------------------------------------------
// ListForHousehold
// ---------------------------------------------------------------------------

func TestAccountLogic_ListForHousehold_ReturnsOnlyHouseholdAccounts(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	_, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "A1", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)
	_, err = logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "A2", AccountType: types.AccountTypeSavings, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	list, err := logic.ListForHousehold(context.Background(), householdID)

	require.NoError(t, err)
	assert.Len(t, list, 2)
}

// ---------------------------------------------------------------------------
// UpdateName
// ---------------------------------------------------------------------------

func TestAccountLogic_UpdateName_HappyPath(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Old", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	updated, err := logic.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: "New"}, created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestAccountLogic_UpdateName_EmptyName_FailsValidation(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Old", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: ""}, created.Version, callerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestAccountLogic_UpdateName_VersionConflict(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Old", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateName(context.Background(), created.ID,
		account.UpdateNameInput{Name: "New"}, created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// ---------------------------------------------------------------------------
// UpdateBalance
// ---------------------------------------------------------------------------

func TestAccountLogic_UpdateBalance_HappyPath(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	updated, err := logic.UpdateBalance(context.Background(), created.ID,
		account.UpdateBalanceInput{Balance: 50000}, created.Version, callerID)

	require.NoError(t, err)
	assert.Equal(t, int64(50000), updated.Balance)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestAccountLogic_UpdateBalance_VersionConflict(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateBalance(context.Background(), created.ID,
		account.UpdateBalanceInput{Balance: 50000}, created.Version+99, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountVersionConflict))
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestAccountLogic_Deactivate_ZeroBalance_Succeeds(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	err = logic.Deactivate(context.Background(), created.ID, callerID)
	require.NoError(t, err)

	_, err = logic.FindByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, message.ErrAccountNotFound))
}

func TestAccountLogic_Deactivate_NonZeroBalance_Fails(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	_, err = logic.UpdateBalance(context.Background(), created.ID,
		account.UpdateBalanceInput{Balance: 10000}, created.Version, callerID)
	require.NoError(t, err)

	err = logic.Deactivate(context.Background(), created.ID, callerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrAccountBalanceNotZero))
}

func TestAccountLogic_Deactivate_IsIdempotent(t *testing.T) {
	logic, _, householdID, callerID := newAccountLogic()

	created, err := logic.Create(context.Background(), householdID, account.CreateInput{
		Name: "Checking", AccountType: types.AccountTypeChecking, CurrencyCode: types.CurrencyUSD,
	}, callerID)
	require.NoError(t, err)

	require.NoError(t, logic.Deactivate(context.Background(), created.ID, callerID))
	assert.NoError(t, logic.Deactivate(context.Background(), created.ID, callerID))
}

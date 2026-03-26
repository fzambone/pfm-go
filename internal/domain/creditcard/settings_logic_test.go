package creditcard_test

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
	"github.com/zambone/pfm-go/internal/platform/clock"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// fakeAccountTypeFinder is a test double that returns a configurable account type.
type fakeAccountTypeFinder struct {
	accountType types.AccountType
	err         error
}

func (f *fakeAccountTypeFinder) FindAccountType(_ context.Context, _ uuid.UUID) (types.AccountType, error) {
	return f.accountType, f.err
}

func newSettingsLogic(accountType types.AccountType) (*creditcard.SettingsLogic, *postgres.FakeCreditCardSettingsRepository) {
	repo := postgres.NewFakeCreditCardSettingsRepository()
	clk := clock.NewFakeClock(fixedTime)
	finder := &fakeAccountTypeFinder{accountType: accountType}
	logic := creditcard.NewSettingsLogic(repo, finder, clk)
	return logic, repo
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestSettingsLogic_Create_CreditCardAccount_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	s, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, 25, s.ClosingDay)
	assert.Equal(t, 10, s.DueDay)
	assert.Equal(t, int64(500000), s.LimitAmount)
}

func TestSettingsLogic_Create_NonCreditCardAccount_Fails(t *testing.T) {
	nonCCTypes := []types.AccountType{
		types.AccountTypeChecking,
		types.AccountTypeSavings,
		types.AccountTypeInvestment,
	}
	for _, at := range nonCCTypes {
		t.Run(string(at), func(t *testing.T) {
			logic, _ := newSettingsLogic(at)

			_, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)

			require.Error(t, err)
			assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotCreditCard))
		})
	}
}

func TestSettingsLogic_Create_DuplicateSettings_Fails(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	_, err = logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsExists))
}

func TestSettingsLogic_Create_ClosingDayOutOfRange_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	tests := []struct {
		name string
		day  int
	}{
		{"zero", 0},
		{"negative", -1},
		{"too high", 32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := logic.Create(context.Background(), testAccountID,
				creditcard.CreateInput{ClosingDay: tc.day, DueDay: 10, LimitAmount: 500000}, testCallerID)

			require.Error(t, err)
			var ve *validate.ValidationError
			assert.True(t, errors.As(err, &ve))
		})
	}
}

func TestSettingsLogic_Create_DueDayOutOfRange_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.Create(context.Background(), testAccountID,
		creditcard.CreateInput{ClosingDay: 25, DueDay: 0, LimitAmount: 500000}, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestSettingsLogic_Create_NegativeLimit_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.Create(context.Background(), testAccountID,
		creditcard.CreateInput{ClosingDay: 25, DueDay: 10, LimitAmount: -1}, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestSettingsLogic_Create_ZeroLimit_Accepted(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	s, err := logic.Create(context.Background(), testAccountID,
		creditcard.CreateInput{ClosingDay: 25, DueDay: 10, LimitAmount: 0}, testCallerID)

	require.NoError(t, err)
	assert.Equal(t, int64(0), s.LimitAmount)
}

func TestSettingsLogic_Create_Day31_Accepted(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	s, err := logic.Create(context.Background(), testAccountID,
		creditcard.CreateInput{ClosingDay: 31, DueDay: 31, LimitAmount: 500000}, testCallerID)

	require.NoError(t, err)
	assert.Equal(t, 31, s.ClosingDay)
	assert.Equal(t, 31, s.DueDay)
}

// ---------------------------------------------------------------------------
// FindByAccountID
// ---------------------------------------------------------------------------

func TestSettingsLogic_FindByAccountID_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	found, err := logic.FindByAccountID(context.Background(), testAccountID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestSettingsLogic_FindByAccountID_NotFound(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.FindByAccountID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

// ---------------------------------------------------------------------------
// UpdateClosingDay
// ---------------------------------------------------------------------------

func TestSettingsLogic_UpdateClosingDay_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	updated, err := logic.UpdateClosingDay(context.Background(), testAccountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, created.Version, testCallerID)

	require.NoError(t, err)
	assert.Equal(t, 15, updated.ClosingDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestSettingsLogic_UpdateClosingDay_OutOfRange_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	_, err = logic.UpdateClosingDay(context.Background(), testAccountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 0}, created.Version, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

func TestSettingsLogic_UpdateClosingDay_VersionConflict(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	_, err = logic.UpdateClosingDay(context.Background(), testAccountID,
		creditcard.UpdateClosingDayInput{ClosingDay: 15}, 99, testCallerID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsVersionConflict))
}

// ---------------------------------------------------------------------------
// UpdateDueDay
// ---------------------------------------------------------------------------

func TestSettingsLogic_UpdateDueDay_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	updated, err := logic.UpdateDueDay(context.Background(), testAccountID,
		creditcard.UpdateDueDayInput{DueDay: 5}, created.Version, testCallerID)

	require.NoError(t, err)
	assert.Equal(t, 5, updated.DueDay)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestSettingsLogic_UpdateDueDay_OutOfRange_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	_, err = logic.UpdateDueDay(context.Background(), testAccountID,
		creditcard.UpdateDueDayInput{DueDay: 32}, created.Version, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

// ---------------------------------------------------------------------------
// UpdateLimit
// ---------------------------------------------------------------------------

func TestSettingsLogic_UpdateLimit_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	updated, err := logic.UpdateLimit(context.Background(), testAccountID,
		creditcard.UpdateLimitInput{LimitAmount: 1000000}, created.Version, testCallerID)

	require.NoError(t, err)
	assert.Equal(t, int64(1000000), updated.LimitAmount)
	assert.Equal(t, created.Version+1, updated.Version)
}

func TestSettingsLogic_UpdateLimit_Negative_FailsValidation(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	created, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	_, err = logic.UpdateLimit(context.Background(), testAccountID,
		creditcard.UpdateLimitInput{LimitAmount: -1}, created.Version, testCallerID)

	require.Error(t, err)
	var ve *validate.ValidationError
	assert.True(t, errors.As(err, &ve))
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestSettingsLogic_Delete_Succeeds(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	_, err := logic.Create(context.Background(), testAccountID, createInputFactory(), testCallerID)
	require.NoError(t, err)

	err = logic.Delete(context.Background(), testAccountID, testCallerID)
	require.NoError(t, err)

	_, err = logic.FindByAccountID(context.Background(), testAccountID)
	assert.True(t, errors.Is(err, message.ErrCreditCardSettingsNotFound))
}

func TestSettingsLogic_Delete_IsIdempotent(t *testing.T) {
	logic, _ := newSettingsLogic(types.AccountTypeCreditCard)

	err := logic.Delete(context.Background(), uuid.New(), testCallerID)
	assert.NoError(t, err)
}

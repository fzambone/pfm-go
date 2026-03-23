package account_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/types"
)

var (
	fixedTime       = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	testAccountID   = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	testHouseholdID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	testCallerID    = uuid.MustParse("00000000-0000-0000-0000-000000000001")
)

// accountFactory returns an Account with all required fields set to non-zero defaults.
// Individual tests override only the fields relevant to their scenario:
//
//	a := accountFactory(func(a *account.Account) { a.Name = "Custom" })
func accountFactory(overrides ...func(*account.Account)) account.Account {
	a := account.Account{
		ID:           testAccountID,
		HouseholdID:  testHouseholdID,
		Name:         "Test Checking",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
		Balance:      0,
		Status:       types.StatusActive,
		Version:      1,
		CreatedAt:    fixedTime,
		UpdatedAt:    fixedTime,
		CreatedBy:    testCallerID,
		UpdatedBy:    testCallerID,
	}
	for _, o := range overrides {
		o(&a)
	}
	return a
}

// createInputFactory returns a valid CreateInput with sensible defaults.
func createInputFactory(overrides ...func(*account.CreateInput)) account.CreateInput {
	in := account.CreateInput{
		Name:         "New Account",
		AccountType:  types.AccountTypeChecking,
		CurrencyCode: types.CurrencyUSD,
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateNameInputFactory returns a valid UpdateNameInput with sensible defaults.
func updateNameInputFactory(overrides ...func(*account.UpdateNameInput)) account.UpdateNameInput {
	in := account.UpdateNameInput{
		Name: "Updated Account",
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// updateBalanceInputFactory returns a valid UpdateBalanceInput with sensible defaults.
func updateBalanceInputFactory(overrides ...func(*account.UpdateBalanceInput)) account.UpdateBalanceInput {
	in := account.UpdateBalanceInput{
		Balance: 10000, // $100.00 in cents
	}
	for _, o := range overrides {
		o(&in)
	}
	return in
}

// TestFactories_ProduceValidDefaults verifies that each factory returns fully
// populated values when called with no overrides.
func TestFactories_ProduceValidDefaults(t *testing.T) {
	t.Run("accountFactory has non-zero fields", func(t *testing.T) {
		a := accountFactory()

		assert.NotEqual(t, uuid.Nil, a.ID)
		assert.NotEqual(t, uuid.Nil, a.HouseholdID)
		assert.NotEmpty(t, a.Name)
		assert.Equal(t, types.AccountTypeChecking, a.AccountType)
		assert.Equal(t, types.CurrencyUSD, a.CurrencyCode)
		assert.Equal(t, int64(0), a.Balance)
		assert.Equal(t, types.StatusActive, a.Status)
		assert.Equal(t, 1, a.Version)
		assert.False(t, a.CreatedAt.IsZero())
		assert.False(t, a.UpdatedAt.IsZero())
		assert.NotEqual(t, uuid.Nil, a.CreatedBy)
	})

	t.Run("accountFactory override applies", func(t *testing.T) {
		a := accountFactory(func(a *account.Account) { a.Name = "Custom" })

		assert.Equal(t, "Custom", a.Name)
	})

	t.Run("accountFactory fixedTime is 2026-01-01", func(t *testing.T) {
		a := accountFactory()

		assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), a.CreatedAt)
	})

	t.Run("createInputFactory has valid defaults", func(t *testing.T) {
		in := createInputFactory()

		assert.NotEmpty(t, in.Name)
		assert.Equal(t, types.AccountTypeChecking, in.AccountType)
		assert.Equal(t, types.CurrencyUSD, in.CurrencyCode)
	})

	t.Run("updateNameInputFactory has non-empty name", func(t *testing.T) {
		in := updateNameInputFactory()

		assert.NotEmpty(t, in.Name)
	})

	t.Run("updateBalanceInputFactory has non-zero balance", func(t *testing.T) {
		in := updateBalanceInputFactory()

		assert.NotEqual(t, int64(0), in.Balance)
	})
}

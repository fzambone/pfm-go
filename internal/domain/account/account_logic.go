package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/validate"
	"github.com/zambone/pfm-go/internal/types"
)

// clocker abstracts the current time.
// Structurally satisfied by platform/clock.Clock.
type clocker interface {
	Now() time.Time
}

// validAccountTypes lists the allowed values for the account_type validation rule.
var validAccountTypes = []string{
	string(types.AccountTypeChecking),
	string(types.AccountTypeSavings),
	string(types.AccountTypeCreditCard),
	string(types.AccountTypeInvestment),
}

// validCurrencies lists the allowed values for the currency_code validation rule.
var validCurrencies = []string{
	string(types.CurrencyUSD),
	string(types.CurrencyBRL),
	string(types.CurrencyEUR),
}

// AccountLogic orchestrates the account lifecycle: creation, queries,
// name and balance updates, and deactivation with zero-balance enforcement.
type AccountLogic struct {
	repo Repository
	clk  clocker
}

// NewAccountLogic constructs an AccountLogic. Panics if any dependency is nil.
func NewAccountLogic(repo Repository, clk clocker) *AccountLogic {
	if repo == nil {
		panic("account: NewAccountLogic requires non-nil repo")
	}
	if clk == nil {
		panic("account: NewAccountLogic requires non-nil clk")
	}
	return &AccountLogic{repo: repo, clk: clk}
}

// Create validates input and persists a new account with zero balance.
func (l *AccountLogic) Create(ctx context.Context, householdID uuid.UUID, input CreateInput, callerID uuid.UUID) (Account, error) {
	r := validate.NewResult()
	r.Field("name", input.Name, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	r.Field("account_type", string(input.AccountType), validate.Required, validate.OneOf(validAccountTypes...))
	r.Field("currency_code", string(input.CurrencyCode), validate.Required, validate.OneOf(validCurrencies...))
	if err := r.Error(); err != nil {
		return Account{}, err
	}

	a, err := l.repo.Create(ctx, householdID, input, callerID)
	if err != nil {
		if errors.Is(err, message.ErrAccountNameTaken) {
			return Account{}, fmt.Errorf(message.ErrAccountLogicCreate, message.ErrAccountNameTaken)
		}
		return Account{}, fmt.Errorf(message.ErrAccountLogicCreate, err)
	}
	return a, nil
}

// FindByID returns the active account with the given ID.
func (l *AccountLogic) FindByID(ctx context.Context, id uuid.UUID) (Account, error) {
	a, err := l.repo.FindByID(ctx, id)
	if err != nil {
		return Account{}, fmt.Errorf(message.ErrAccountLogicFindByID, err)
	}
	return a, nil
}

// ListForHousehold returns all active accounts belonging to the given household.
func (l *AccountLogic) ListForHousehold(ctx context.Context, householdID uuid.UUID) ([]Account, error) {
	list, err := l.repo.ListForHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf(message.ErrAccountLogicListForHouse, err)
	}
	return list, nil
}

// UpdateName validates input and updates the account name with optimistic concurrency.
func (l *AccountLogic) UpdateName(ctx context.Context, id uuid.UUID, input UpdateNameInput, expectedVersion int, callerID uuid.UUID) (Account, error) {
	r := validate.NewResult()
	r.Field("name", input.Name, validate.Required, validate.MinLen(2), validate.MaxLen(100))
	if err := r.Error(); err != nil {
		return Account{}, err
	}

	a, err := l.repo.UpdateName(ctx, id, input, expectedVersion, callerID)
	if err != nil {
		return Account{}, fmt.Errorf(message.ErrAccountLogicUpdateName, err)
	}
	return a, nil
}

// UpdateBalance sets the account balance to the new value with optimistic concurrency.
func (l *AccountLogic) UpdateBalance(ctx context.Context, id uuid.UUID, input UpdateBalanceInput, expectedVersion int, callerID uuid.UUID) (Account, error) {
	a, err := l.repo.UpdateBalance(ctx, id, input, expectedVersion, callerID)
	if err != nil {
		return Account{}, fmt.Errorf(message.ErrAccountLogicUpdateBalance, err)
	}
	return a, nil
}

// Deactivate checks the account has zero balance, then soft-deletes it.
// Returns ErrAccountBalanceNotZero if the balance is non-zero.
func (l *AccountLogic) Deactivate(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	a, err := l.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, message.ErrAccountNotFound) {
			return nil // already gone — idempotent
		}
		return fmt.Errorf(message.ErrAccountLogicDeactivate, err)
	}

	if a.Balance != 0 {
		return fmt.Errorf(message.ErrAccountLogicDeactivate, message.ErrAccountBalanceNotZero)
	}

	if err := l.repo.Deactivate(ctx, id, callerID); err != nil {
		return fmt.Errorf(message.ErrAccountLogicDeactivate, err)
	}
	return nil
}

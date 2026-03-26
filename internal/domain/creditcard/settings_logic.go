package creditcard

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

// accountTypeFinder abstracts the account type lookup needed to verify that
// settings are only created for credit card accounts. Structurally satisfied
// by a thin adapter wrapping account.Repository.FindByID.
type accountTypeFinder interface {
	FindAccountType(ctx context.Context, accountID uuid.UUID) (types.AccountType, error)
}

// clocker abstracts the current time.
// Structurally satisfied by platform/clock.Clock.
type clocker interface {
	Now() time.Time
}

// SettingsLogic orchestrates the credit card settings lifecycle: creation with
// account-type verification, queries, field updates, and deletion.
type SettingsLogic struct {
	repo    Repository
	acctFdr accountTypeFinder
	clk     clocker
}

// NewSettingsLogic constructs a SettingsLogic. Panics if any dependency is nil.
func NewSettingsLogic(repo Repository, acctFdr accountTypeFinder, clk clocker) *SettingsLogic {
	if repo == nil {
		panic("creditcard: NewSettingsLogic requires non-nil repo")
	}
	if acctFdr == nil {
		panic("creditcard: NewSettingsLogic requires non-nil accountTypeFinder")
	}
	if clk == nil {
		panic("creditcard: NewSettingsLogic requires non-nil clk")
	}
	return &SettingsLogic{repo: repo, acctFdr: acctFdr, clk: clk}
}

// validateDayAndLimit validates closing day (1-31), due day (1-31), and limit (>= 0).
func validateDayAndLimit(closingDay, dueDay int, limitAmount int64) error {
	r := validate.NewResult()
	r.Field("closing_day", closingDay, validate.Range(1, 31))
	r.Field("due_day", dueDay, validate.Range(1, 31))
	r.Field("limit_amount", limitAmount, validate.NonNegative)
	return r.Error()
}

// Create verifies the account is a credit card, validates input, and persists new settings.
func (l *SettingsLogic) Create(ctx context.Context, accountID uuid.UUID, input CreateInput, callerID uuid.UUID) (Settings, error) {
	acctType, err := l.acctFdr.FindAccountType(ctx, accountID)
	if err != nil {
		return Settings{}, fmt.Errorf(message.ErrCCLogicCreate, err)
	}
	if acctType != types.AccountTypeCreditCard {
		return Settings{}, fmt.Errorf(message.ErrCCLogicCreate, message.ErrCreditCardSettingsNotCreditCard)
	}

	if err := validateDayAndLimit(input.ClosingDay, input.DueDay, input.LimitAmount); err != nil {
		return Settings{}, err
	}

	s, err := l.repo.Create(ctx, accountID, input, callerID)
	if err != nil {
		if errors.Is(err, message.ErrCreditCardSettingsExists) {
			return Settings{}, fmt.Errorf(message.ErrCCLogicCreate, message.ErrCreditCardSettingsExists)
		}
		return Settings{}, fmt.Errorf(message.ErrCCLogicCreate, err)
	}
	return s, nil
}

// FindByAccountID returns the active settings for the given account.
func (l *SettingsLogic) FindByAccountID(ctx context.Context, accountID uuid.UUID) (Settings, error) {
	s, err := l.repo.FindByAccountID(ctx, accountID)
	if err != nil {
		return Settings{}, fmt.Errorf(message.ErrCCLogicFindByAccount, err)
	}
	return s, nil
}

// UpdateClosingDay validates and changes the billing cycle closing day.
func (l *SettingsLogic) UpdateClosingDay(ctx context.Context, accountID uuid.UUID, input UpdateClosingDayInput, expectedVersion int, callerID uuid.UUID) (Settings, error) {
	r := validate.NewResult()
	r.Field("closing_day", input.ClosingDay, validate.Range(1, 31))
	if err := r.Error(); err != nil {
		return Settings{}, err
	}

	s, err := l.repo.UpdateClosingDay(ctx, accountID, input, expectedVersion, callerID)
	if err != nil {
		return Settings{}, fmt.Errorf(message.ErrCCLogicUpdateClosing, err)
	}
	return s, nil
}

// UpdateDueDay validates and changes the payment due day.
func (l *SettingsLogic) UpdateDueDay(ctx context.Context, accountID uuid.UUID, input UpdateDueDayInput, expectedVersion int, callerID uuid.UUID) (Settings, error) {
	r := validate.NewResult()
	r.Field("due_day", input.DueDay, validate.Range(1, 31))
	if err := r.Error(); err != nil {
		return Settings{}, err
	}

	s, err := l.repo.UpdateDueDay(ctx, accountID, input, expectedVersion, callerID)
	if err != nil {
		return Settings{}, fmt.Errorf(message.ErrCCLogicUpdateDueDay, err)
	}
	return s, nil
}

// UpdateLimit validates and changes the credit limit.
func (l *SettingsLogic) UpdateLimit(ctx context.Context, accountID uuid.UUID, input UpdateLimitInput, expectedVersion int, callerID uuid.UUID) (Settings, error) {
	r := validate.NewResult()
	r.Field("limit_amount", input.LimitAmount, validate.NonNegative)
	if err := r.Error(); err != nil {
		return Settings{}, err
	}

	s, err := l.repo.UpdateLimit(ctx, accountID, input, expectedVersion, callerID)
	if err != nil {
		return Settings{}, fmt.Errorf(message.ErrCCLogicUpdateLimit, err)
	}
	return s, nil
}

// Delete soft-deletes the settings for the given account.
func (l *SettingsLogic) Delete(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error {
	if err := l.repo.Delete(ctx, accountID, callerID); err != nil {
		return fmt.Errorf(message.ErrCCLogicDelete, err)
	}
	return nil
}

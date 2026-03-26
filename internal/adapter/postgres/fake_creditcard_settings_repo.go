package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/domain/creditcard"
	"github.com/zambone/pfm-go/internal/message"
)

// FakeCreditCardSettingsRepository is a test double for domain/creditcard.Repository.
// It stores settings in an in-memory map keyed by account ID (one-to-one relationship).
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeCreditCardSettingsRepository struct {
	mu        sync.RWMutex
	byAccount map[uuid.UUID]creditcard.Settings // key: AccountID
	err       error                             // injected error returned by all methods
}

// NewFakeCreditCardSettingsRepository returns an empty repository ready for use in tests.
func NewFakeCreditCardSettingsRepository() *FakeCreditCardSettingsRepository {
	return &FakeCreditCardSettingsRepository{
		byAccount: make(map[uuid.UUID]creditcard.Settings),
	}
}

// SetError configures every subsequent method call to return err.
// Pass nil to clear the injected error.
func (f *FakeCreditCardSettingsRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Create stores new credit card settings for the given account.
// Returns an error wrapping ErrCreditCardSettingsExists if settings already exist.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) Create(_ context.Context, accountID uuid.UUID, input creditcard.CreateInput, callerID uuid.UUID) (creditcard.Settings, error) {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return creditcard.Settings{}, f.err
	}

	if _, exists := f.byAccount[accountID]; exists {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsCreate, message.ErrCreditCardSettingsExists)
	}

	s := creditcard.Settings{
		ID:          uuid.New(),
		AccountID:   accountID,
		ClosingDay:  input.ClosingDay,
		DueDay:      input.DueDay,
		LimitAmount: input.LimitAmount,
		Version:     1,
		CreatedBy:   callerID,
		UpdatedBy:   callerID,
	}
	f.byAccount[accountID] = s
	return s, nil
}

// FindByAccountID returns the active settings for the given account.
// Returns an error wrapping ErrCreditCardSettingsNotFound when not found.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) FindByAccountID(_ context.Context, accountID uuid.UUID) (creditcard.Settings, error) {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return creditcard.Settings{}, f.err
	}

	s, ok := f.byAccount[accountID]
	if !ok {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsFindByAccount, message.ErrCreditCardSettingsNotFound)
	}
	return s, nil
}

// UpdateClosingDay changes the billing cycle closing day.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict if version mismatches.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) UpdateClosingDay(_ context.Context, accountID uuid.UUID, input creditcard.UpdateClosingDayInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return creditcard.Settings{}, f.err
	}

	s, ok := f.byAccount[accountID]
	if !ok {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateClosing, message.ErrCreditCardSettingsNotFound)
	}
	if s.Version != expectedVersion {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateClosing, message.ErrCreditCardSettingsVersionConflict)
	}

	s.ClosingDay = input.ClosingDay
	s.Version++
	s.UpdatedBy = callerID
	f.byAccount[accountID] = s
	return s, nil
}

// UpdateDueDay changes the payment due day.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict if version mismatches.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) UpdateDueDay(_ context.Context, accountID uuid.UUID, input creditcard.UpdateDueDayInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return creditcard.Settings{}, f.err
	}

	s, ok := f.byAccount[accountID]
	if !ok {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateDueDay, message.ErrCreditCardSettingsNotFound)
	}
	if s.Version != expectedVersion {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateDueDay, message.ErrCreditCardSettingsVersionConflict)
	}

	s.DueDay = input.DueDay
	s.Version++
	s.UpdatedBy = callerID
	f.byAccount[accountID] = s
	return s, nil
}

// UpdateLimit changes the credit limit.
// Returns an error wrapping ErrCreditCardSettingsVersionConflict if version mismatches.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) UpdateLimit(_ context.Context, accountID uuid.UUID, input creditcard.UpdateLimitInput, expectedVersion int, callerID uuid.UUID) (creditcard.Settings, error) {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return creditcard.Settings{}, f.err
	}

	s, ok := f.byAccount[accountID]
	if !ok {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateLimit, message.ErrCreditCardSettingsNotFound)
	}
	if s.Version != expectedVersion {
		return creditcard.Settings{}, fmt.Errorf(message.ErrCCSettingsUpdateLimit, message.ErrCreditCardSettingsVersionConflict)
	}

	s.LimitAmount = input.LimitAmount
	s.Version++
	s.UpdatedBy = callerID
	f.byAccount[accountID] = s
	return s, nil
}

// Delete soft-deletes the settings for the given account.
// Idempotent — deleting non-existent settings is not an error.
// Panics if called outside a test binary.
func (f *FakeCreditCardSettingsRepository) Delete(_ context.Context, accountID uuid.UUID, _ uuid.UUID) error {
	if !testing.Testing() {
		panic("FakeCreditCardSettingsRepository: not for production use")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	delete(f.byAccount, accountID)
	return nil
}

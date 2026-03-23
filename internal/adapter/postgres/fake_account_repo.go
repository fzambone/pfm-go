package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/domain/account"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

// FakeAccountRepository is a test double for domain/account.Repository.
// It stores accounts in an in-memory map keyed by ID and enforces per-household
// name uniqueness via a secondary index.
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeAccountRepository struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]account.Account
	names map[string]uuid.UUID // key: "householdID:lower(name)" → account ID
	err   error                // injected error returned by all methods
}

// NewFakeAccountRepository returns an empty FakeAccountRepository ready for use in tests.
func NewFakeAccountRepository() *FakeAccountRepository {
	return &FakeAccountRepository{
		byID:  make(map[uuid.UUID]account.Account),
		names: make(map[string]uuid.UUID),
	}
}

// nameKey builds the composite key for the per-household name uniqueness index.
func nameKey(householdID uuid.UUID, name string) string {
	return householdID.String() + ":" + strings.ToLower(name)
}

// SetError configures every subsequent method call to return err.
// Pass nil to clear the injected error.
func (f *FakeAccountRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Create stores a new account with zero balance and returns it.
// Returns an error wrapping ErrAccountNameTaken if the name already exists in the household.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) Create(_ context.Context, householdID uuid.UUID, input account.CreateInput, callerID uuid.UUID) (account.Account, error) {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return account.Account{}, f.err
	}

	nk := nameKey(householdID, input.Name)
	if _, exists := f.names[nk]; exists {
		return account.Account{}, fmt.Errorf(message.ErrAccountCreate, message.ErrAccountNameTaken)
	}

	a := account.Account{
		ID:           uuid.New(),
		HouseholdID:  householdID,
		Name:         input.Name,
		AccountType:  input.AccountType,
		CurrencyCode: input.CurrencyCode,
		Balance:      0,
		Status:       types.StatusActive,
		Version:      1,
		CreatedBy:    callerID,
		UpdatedBy:    callerID,
	}
	f.byID[a.ID] = a
	f.names[nk] = a.ID
	return a, nil
}

// FindByID returns the active account with the given ID.
// Returns an error wrapping ErrAccountNotFound when not found.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) FindByID(_ context.Context, id uuid.UUID) (account.Account, error) {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return account.Account{}, f.err
	}

	a, ok := f.byID[id]
	if !ok {
		return account.Account{}, fmt.Errorf(message.ErrAccountFindByID, message.ErrAccountNotFound)
	}
	return a, nil
}

// ListForHousehold returns all active accounts belonging to the given household.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) ListForHousehold(_ context.Context, householdID uuid.UUID) ([]account.Account, error) {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return nil, f.err
	}

	var result []account.Account
	for _, a := range f.byID {
		if a.HouseholdID == householdID {
			result = append(result, a)
		}
	}
	return result, nil
}

// UpdateName changes the name of the account.
// Returns an error wrapping ErrAccountVersionConflict if expectedVersion does not match.
// Returns an error wrapping ErrAccountNameTaken if the new name conflicts.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) UpdateName(_ context.Context, id uuid.UUID, input account.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (account.Account, error) {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return account.Account{}, f.err
	}

	a, ok := f.byID[id]
	if !ok {
		return account.Account{}, fmt.Errorf(message.ErrAccountUpdateName, message.ErrAccountNotFound)
	}
	if a.Version != expectedVersion {
		return account.Account{}, fmt.Errorf(message.ErrAccountUpdateName, message.ErrAccountVersionConflict)
	}

	newKey := nameKey(a.HouseholdID, input.Name)
	if existingID, exists := f.names[newKey]; exists && existingID != id {
		return account.Account{}, fmt.Errorf(message.ErrAccountUpdateName, message.ErrAccountNameTaken)
	}

	oldKey := nameKey(a.HouseholdID, a.Name)
	delete(f.names, oldKey)

	a.Name = input.Name
	a.Version++
	a.UpdatedBy = callerID
	f.byID[id] = a
	f.names[newKey] = id
	return a, nil
}

// UpdateBalance sets the account balance to the new value.
// Returns an error wrapping ErrAccountVersionConflict if expectedVersion does not match.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) UpdateBalance(_ context.Context, id uuid.UUID, input account.UpdateBalanceInput, expectedVersion int, callerID uuid.UUID) (account.Account, error) {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return account.Account{}, f.err
	}

	a, ok := f.byID[id]
	if !ok {
		return account.Account{}, fmt.Errorf(message.ErrAccountUpdateBalance, message.ErrAccountNotFound)
	}
	if a.Version != expectedVersion {
		return account.Account{}, fmt.Errorf(message.ErrAccountUpdateBalance, message.ErrAccountVersionConflict)
	}

	a.Balance = input.Balance
	a.Version++
	a.UpdatedBy = callerID
	f.byID[id] = a
	return a, nil
}

// Deactivate soft-deletes the account by removing it from the maps.
// Idempotent — deactivating an already-removed account is not an error.
// Panics if called outside a test binary.
func (f *FakeAccountRepository) Deactivate(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	if !testing.Testing() {
		panic("FakeAccountRepository: not for production use — wire AccountRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	a, ok := f.byID[id]
	if !ok {
		return nil // already gone — idempotent
	}
	delete(f.names, nameKey(a.HouseholdID, a.Name))
	delete(f.byID, id)
	return nil
}

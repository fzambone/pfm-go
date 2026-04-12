package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/domain/household"
	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/types"
)

// memberKey uniquely identifies a membership by household and user.
type memberKey struct {
	HouseholdID uuid.UUID
	UserID      uuid.UUID
}

// FakeHouseholdRepository is a test double for domain/household.Repository.
// It stores households in an in-memory map and memberships in a separate map
// keyed by (household_id, user_id).
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeHouseholdRepository struct {
	mu      sync.RWMutex
	byID    map[uuid.UUID]household.Household   // key: Household.ID
	members map[memberKey]household.Membership   // key: (household_id, user_id)
	removed map[memberKey]bool                   // tracks soft-deleted memberships
	err     error                                // injected error returned by all methods
}

// NewFakeHouseholdRepository returns an empty FakeHouseholdRepository ready for use in tests.
func NewFakeHouseholdRepository() *FakeHouseholdRepository {
	return &FakeHouseholdRepository{
		byID:    make(map[uuid.UUID]household.Household),
		members: make(map[memberKey]household.Membership),
		removed: make(map[memberKey]bool),
	}
}

// AddHousehold seeds a household with the given ID directly into the in-memory store.
// Use this in tests to set up preconditions without going through the domain Create method.
func (f *FakeHouseholdRepository) AddHousehold(id uuid.UUID, callerID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[id] = household.Household{
		ID:        id,
		Name:      "test household",
		Status:    "ACTIVE",
		Version:   1,
		CreatedBy: callerID,
		UpdatedBy: callerID,
	}
}

// AddMemberDirect seeds a membership directly into the in-memory store.
// Use this in tests to set up preconditions (e.g. to trigger ErrHouseholdMemberExists).
func (f *FakeHouseholdRepository) AddMemberDirect(householdID, userID, inviterID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := memberKey{HouseholdID: householdID, UserID: userID}
	f.members[key] = household.Membership{
		HouseholdID: householdID,
		UserID:      userID,
		Role:        "MEMBER",
		InvitedBy:   inviterID,
	}
}

// SetError configures every subsequent method call to return err.
// Pass nil to clear the injected error.
func (f *FakeHouseholdRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Create stores a new household and its founding admin membership atomically.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) Create(_ context.Context, input household.CreateInput, callerID uuid.UUID) (household.Household, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return household.Household{}, f.err
	}

	h := household.Household{
		ID:        uuid.New(),
		Name:      input.Name,
		Status:    types.StatusActive,
		Version:   1,
		CreatedBy: callerID,
		UpdatedBy: callerID,
	}
	f.byID[h.ID] = h

	key := memberKey{HouseholdID: h.ID, UserID: callerID}
	f.members[key] = household.Membership{
		HouseholdID: h.ID,
		UserID:      callerID,
		Role:        types.RoleAdmin,
		InvitedBy:   uuid.Nil,
	}

	return h, nil
}

// FindByID returns the active household with the given ID.
// Returns an error wrapping ErrHouseholdNotFound when not found.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) FindByID(_ context.Context, id uuid.UUID) (household.Household, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return household.Household{}, f.err
	}

	h, ok := f.byID[id]
	if !ok {
		return household.Household{}, fmt.Errorf(message.ErrHouseholdFindByID, message.ErrHouseholdNotFound)
	}
	return h, nil
}

// ListForUser returns all active households where the given user has an active membership.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) ListForUser(_ context.Context, userID uuid.UUID) ([]household.Household, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return nil, f.err
	}

	var result []household.Household
	for key, m := range f.members {
		if m.UserID == userID && !f.removed[key] {
			if h, ok := f.byID[key.HouseholdID]; ok {
				result = append(result, h)
			}
		}
	}
	return result, nil
}

// FindMembership returns the active membership for a user in a household.
// Returns an error wrapping ErrHouseholdMemberNotFound when no active membership exists.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) FindMembership(_ context.Context, householdID uuid.UUID, userID uuid.UUID) (household.Membership, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return household.Membership{}, f.err
	}

	key := memberKey{HouseholdID: householdID, UserID: userID}
	m, ok := f.members[key]
	if !ok || f.removed[key] {
		return household.Membership{}, fmt.Errorf(message.ErrHouseholdFindMembership, message.ErrHouseholdMemberNotFound)
	}
	return m, nil
}

// ListMembers returns all active memberships for the given household.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) ListMembers(_ context.Context, householdID uuid.UUID) ([]household.Membership, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return nil, f.err
	}

	var result []household.Membership
	for key, m := range f.members {
		if key.HouseholdID == householdID && !f.removed[key] {
			result = append(result, m)
		}
	}
	return result, nil
}

// AddMember adds a new membership to the household.
// Returns an error wrapping ErrHouseholdMemberExists if the user already has an active membership.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) AddMember(_ context.Context, householdID uuid.UUID, input household.AddMemberInput, callerID uuid.UUID) (household.Membership, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return household.Membership{}, f.err
	}

	key := memberKey{HouseholdID: householdID, UserID: input.UserID}
	if _, exists := f.members[key]; exists && !f.removed[key] {
		return household.Membership{}, fmt.Errorf(message.ErrHouseholdAddMember, message.ErrHouseholdMemberExists)
	}

	m := household.Membership{
		HouseholdID: householdID,
		UserID:      input.UserID,
		Role:        input.Role,
		InvitedBy:   callerID,
	}
	f.members[key] = m
	delete(f.removed, key) // re-adding a previously removed member clears the soft-delete
	return m, nil
}

// RemoveMember soft-deletes the membership for the given user in the household.
// Idempotent — removing a non-existent membership is not an error.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) RemoveMember(_ context.Context, householdID uuid.UUID, userID uuid.UUID, _ uuid.UUID) error {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	key := memberKey{HouseholdID: householdID, UserID: userID}
	f.removed[key] = true
	return nil
}

// UpdateName changes the name of the household.
// Returns an error wrapping ErrHouseholdVersionConflict if expectedVersion does not match.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) UpdateName(_ context.Context, id uuid.UUID, input household.UpdateNameInput, expectedVersion int, callerID uuid.UUID) (household.Household, error) {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return household.Household{}, f.err
	}

	h, ok := f.byID[id]
	if !ok {
		return household.Household{}, fmt.Errorf(message.ErrHouseholdUpdateName, message.ErrHouseholdNotFound)
	}
	if h.Version != expectedVersion {
		return household.Household{}, fmt.Errorf(message.ErrHouseholdUpdateName, message.ErrHouseholdVersionConflict)
	}

	h.Name = input.Name
	h.Version++
	h.UpdatedBy = callerID
	f.byID[id] = h
	return h, nil
}

// Deactivate soft-deletes the household by removing it from the map.
// Idempotent — deactivating an already-removed household is not an error.
// Panics if called outside a test binary.
func (f *FakeHouseholdRepository) Deactivate(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	if !testing.Testing() {
		panic("FakeHouseholdRepository: not for production use — wire HouseholdRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	delete(f.byID, id)
	return nil
}

package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// FakeUserRepository is a test double for domain/user.Repository.
// It stores users in two in-memory indexes:
//   - byEmail: lower-cased email → User (for case-insensitive FindByEmail)
//   - byID:    UUID → User (for FindByID and mutations)
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeUserRepository struct {
	mu      sync.RWMutex
	byEmail map[string]domainuser.User     // key: lower(email)
	byID    map[uuid.UUID]domainuser.User  // key: User.ID
	err     error                          // injected error returned by all methods
}

// NewFakeUserRepository returns an empty FakeUserRepository ready for use in tests.
func NewFakeUserRepository() *FakeUserRepository {
	return &FakeUserRepository{
		byEmail: make(map[string]domainuser.User),
		byID:    make(map[uuid.UUID]domainuser.User),
	}
}

// Add inserts a user into both indexes. Overwrites any existing entry with the same email or ID.
// If u.Version is zero it is normalised to 1, matching DB DEFAULT behaviour.
func (f *FakeUserRepository) Add(u domainuser.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.Version == 0 {
		u.Version = 1
	}
	f.byEmail[strings.ToLower(u.Email)] = u
	f.byID[u.ID] = u
}

// SetError configures every subsequent method call to return err.
// Pass nil to clear the injected error.
func (f *FakeUserRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Create generates a new UUID, sets Version = 1, stores the user in both indexes, and returns it.
// Panics if called outside a test binary.
func (f *FakeUserRepository) Create(_ context.Context, input domainuser.RegisterInput, passwordHash string, callerID uuid.UUID) (domainuser.User, error) {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return domainuser.User{}, f.err
	}

	if _, exists := f.byEmail[strings.ToLower(input.Email)]; exists {
		return domainuser.User{}, fmt.Errorf(message.ErrUserCreate, message.ErrUserEmailTaken)
	}

	u := domainuser.User{
		ID:           uuid.New(),
		Email:        input.Email,
		DisplayName:  input.DisplayName,
		PasswordHash: passwordHash,
		Version:      1,
		CreatedBy:    callerID,
		UpdatedBy:    callerID,
	}
	f.byEmail[strings.ToLower(u.Email)] = u
	f.byID[u.ID] = u
	return u, nil
}

// FindByID returns the active user with the given ID.
// Returns an error wrapping ErrUserNotFound when not found.
// Panics if called outside a test binary.
func (f *FakeUserRepository) FindByID(_ context.Context, id uuid.UUID) (domainuser.User, error) {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return domainuser.User{}, f.err
	}

	u, ok := f.byID[id]
	if !ok {
		return domainuser.User{}, fmt.Errorf(message.ErrUserFindByID, message.ErrUserNotFound)
	}
	return u, nil
}

// FindByEmail returns the active user matching email (case-insensitive).
// Returns an error wrapping ErrLoginInvalidCredentials when not found.
// Panics if called outside a test binary.
func (f *FakeUserRepository) FindByEmail(_ context.Context, email string) (domainuser.User, error) {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.err != nil {
		return domainuser.User{}, f.err
	}

	u, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return domainuser.User{}, fmt.Errorf(message.ErrUserFindByEmail, message.ErrLoginInvalidCredentials)
	}
	return u, nil
}

// UpdateProfile changes the display name of the user identified by id.
// Returns an error wrapping ErrUserVersionConflict if expectedVersion does not match.
// Panics if called outside a test binary.
func (f *FakeUserRepository) UpdateProfile(_ context.Context, id uuid.UUID, input domainuser.UpdateProfileInput, expectedVersion int, callerID uuid.UUID) (domainuser.User, error) {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return domainuser.User{}, f.err
	}

	u, ok := f.byID[id]
	if !ok {
		return domainuser.User{}, fmt.Errorf(message.ErrUserUpdateProfile, message.ErrUserNotFound)
	}
	if u.Version != expectedVersion {
		return domainuser.User{}, fmt.Errorf(message.ErrUserUpdateProfile, message.ErrUserVersionConflict)
	}

	u.DisplayName = input.DisplayName
	u.Version++
	u.UpdatedBy = callerID
	f.byID[id] = u
	f.byEmail[strings.ToLower(u.Email)] = u
	return u, nil
}

// ChangePassword replaces the password hash of the user identified by id.
// Returns an error wrapping ErrUserVersionConflict if expectedVersion does not match.
// Panics if called outside a test binary.
func (f *FakeUserRepository) ChangePassword(_ context.Context, id uuid.UUID, newHash string, expectedVersion int, callerID uuid.UUID) (domainuser.User, error) {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return domainuser.User{}, f.err
	}

	u, ok := f.byID[id]
	if !ok {
		return domainuser.User{}, fmt.Errorf(message.ErrUserChangePassword, message.ErrUserNotFound)
	}
	if u.Version != expectedVersion {
		return domainuser.User{}, fmt.Errorf(message.ErrUserChangePassword, message.ErrUserVersionConflict)
	}

	u.PasswordHash = newHash
	u.Version++
	u.UpdatedBy = callerID
	f.byID[id] = u
	f.byEmail[strings.ToLower(u.Email)] = u
	return u, nil
}

// Deactivate soft-deletes the user by removing them from both indexes.
// Idempotent — calling with an ID that does not exist (or was already deactivated) returns nil.
// Panics if called outside a test binary.
func (f *FakeUserRepository) Deactivate(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	if !testing.Testing() {
		panic("FakeUserRepository: not for production use — wire UserRepo instead")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	u, ok := f.byID[id]
	if !ok {
		return nil // already gone — idempotent
	}
	delete(f.byID, id)
	delete(f.byEmail, strings.ToLower(u.Email))
	return nil
}

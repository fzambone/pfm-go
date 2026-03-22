package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	domainuser "github.com/zambone/pfm-go/internal/domain/user"
	"github.com/zambone/pfm-go/internal/message"
)

// FakeUserRepository is a test double for domain/user.Repository.
// It stores users in memory keyed by lower-cased email, simulating case-insensitive lookup.
//
// NOT FOR PRODUCTION — panics if called outside a test binary.
// Thread-safe via sync.RWMutex.
type FakeUserRepository struct {
	mu    sync.RWMutex
	users map[string]domainuser.User // key: lower(email)
	err   error                      // injected error returned by FindByEmail (overrides normal lookup)
}

// NewFakeUserRepository returns an empty FakeUserRepository ready for use in tests.
func NewFakeUserRepository() *FakeUserRepository {
	return &FakeUserRepository{
		users: make(map[string]domainuser.User),
	}
}

// Add inserts a user into the fake repository, keyed by lower(email).
// Overwrites any existing entry for the same email.
func (f *FakeUserRepository) Add(u domainuser.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[strings.ToLower(u.Email)] = u
}

// SetError configures FindByEmail to return err on every subsequent call,
// regardless of the stored users. Pass nil to clear the injected error.
func (f *FakeUserRepository) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// FindByEmail returns the user matching email (case-insensitive).
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

	u, ok := f.users[strings.ToLower(email)]
	if !ok {
		return domainuser.User{}, fmt.Errorf(message.ErrUserNotFound, message.ErrLoginInvalidCredentials)
	}
	return u, nil
}

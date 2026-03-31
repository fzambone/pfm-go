package auth

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zambone/pfm-go/internal/message"
	"github.com/zambone/pfm-go/internal/platform/clock"
)

// fakeTokenEntry stores the metadata for a token issued by FakeTokenService.
type fakeTokenEntry struct {
	userID    uuid.UUID
	expiresAt time.Time
}

// FakeTokenService is a test double for port/auth.TokenService.
// It stores issued tokens in memory and uses an injected Clock to control expiry,
// allowing tests to simulate time passing by advancing a FakeClock.
//
// NOT SECURE — never use in production.
// FakeTokenService panics if called outside a test binary (detected via testing.Testing()).
type FakeTokenService struct {
	mu     sync.RWMutex
	clk    clock.Clock
	tokens map[string]fakeTokenEntry
}

// NewFakeTokenService returns a FakeTokenService backed by clk for time control.
// Panics if clk is nil.
func NewFakeTokenService(clk clock.Clock) *FakeTokenService {
	if clk == nil {
		panic("auth: NewFakeTokenService requires non-nil clock")
	}
	return &FakeTokenService{
		clk:    clk,
		tokens: make(map[string]fakeTokenEntry),
	}
}

// Issue creates an opaque token for userID that expires at expiresAt.
// Panics if called outside a test binary.
func (f *FakeTokenService) Issue(_ context.Context, userID uuid.UUID, expiresAt time.Time) (string, error) {
	if !testing.Testing() {
		panic("FakeTokenService: not for production use — wire PasetoTokenService instead")
	}
	token := "fake-" + uuid.New().String()
	entry := fakeTokenEntry{
		userID:    userID,
		expiresAt: expiresAt,
	}
	f.mu.Lock()
	f.tokens[token] = entry
	f.mu.Unlock()
	return token, nil
}

// Validate checks whether token was issued by this service and has not expired.
// Returns ErrTokenExpired if the token is past its expiry, ErrTokenInvalid if unknown.
// Panics if called outside a test binary.
func (f *FakeTokenService) Validate(_ context.Context, token string) (uuid.UUID, error) {
	if !testing.Testing() {
		panic("FakeTokenService: not for production use — wire PasetoTokenService instead")
	}
	f.mu.RLock()
	entry, ok := f.tokens[token]
	f.mu.RUnlock()

	if !ok {
		return uuid.Nil, fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenInvalid)
	}
	if f.clk.Now().After(entry.expiresAt) {
		return uuid.Nil, fmt.Errorf(message.ErrTokenServiceValidate, message.ErrTokenExpired)
	}
	return entry.userID, nil
}

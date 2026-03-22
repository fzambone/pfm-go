package database

import (
	"context"
	"fmt"
	"sync"
)

// FakeTransactor is a test double for Transactor that executes the callback
// synchronously without a real database transaction. It records whether the
// last call committed or rolled back, allowing test assertions on atomicity.
//
// FakeTransactor is safe for concurrent use.
type FakeTransactor struct {
	mu        sync.Mutex
	committed bool
}

// NewFakeTransactor creates a FakeTransactor ready for use in tests.
func NewFakeTransactor() *FakeTransactor {
	return &FakeTransactor{}
}

// RunAtomic calls fn with the same context. If fn returns nil the call is
// recorded as committed; if fn returns an error it is recorded as rolled back.
func (f *FakeTransactor) RunAtomic(ctx context.Context, fn func(ctx context.Context) error) error {
	err := fn(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.committed = err == nil
	if err != nil {
		return fmt.Errorf("transactor: run atomic: %w", err)
	}
	return nil
}

// CommittedLastCall reports whether the last RunAtomic call committed.
// Returns false before any call has been made.
func (f *FakeTransactor) CommittedLastCall() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.committed
}

package clock

import (
	"sync"
	"time"
)

// FakeClock provides a controllable clock for deterministic testing.
// It is safe for concurrent use.
type FakeClock struct {
	mu          sync.RWMutex
	now         time.Time
	initialized bool
	autoAdvance time.Duration
}

// NewFakeClock creates a new FakeClock set to the given time.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t, initialized: true}
}

// Now returns the currently set time.
func (c *FakeClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.initialized {
		panic("clock: FakeClock must be created with NewFakeClock()")
	}
	now := c.now
	if c.autoAdvance > 0 {
		c.now = c.now.Add(c.autoAdvance)
	}
	return now
}

// Set changes the clock to a specific time.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the clock forward by the given duration.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// SetAutoAdvance configure the clock to advance by d on each call to Now.
func (c *FakeClock) SetAutoAdvance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoAdvance = d
}

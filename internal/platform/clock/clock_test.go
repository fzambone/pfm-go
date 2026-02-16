package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zambone/pfm-go/internal/platform/clock"
)

func TestRealClock_Now_ReturnsUTC(t *testing.T) {
	c := clock.NewRealClock()

	got := c.Now()

	assert.Equal(t, time.UTC, got.Location())
}

func TestFakeClock_Now_ReturnsSetTime(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	c := clock.NewFakeClock(fixed)

	got := c.Now()

	assert.Equal(t, fixed, got)
}

func TestFakeClock_Set_ChangesTime(t *testing.T) {
	initial := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	c := clock.NewFakeClock(initial)

	c.Set(updated)

	assert.Equal(t, updated, c.Now())
}

func TestFakeClock_Advance_MovesTimeForward(t *testing.T) {
	initial := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := clock.NewFakeClock(initial)

	c.Advance(2 * time.Hour)

	assert.Equal(t, initial.Add(2*time.Hour), c.Now())
}

func TestFakeClock_Now_ReturnsSameTimeWhenNotAdvanced(t *testing.T) {
	fixed := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	c := clock.NewFakeClock(fixed)

	first := c.Now()
	second := c.Now()

	assert.Equal(t, first, second)
}

func TestFakeClock_ZeroValue_Panics(t *testing.T) {
	assert.Panics(t, func() {
		var c clock.FakeClock
		c.Now()
	})
}
func TestFakeClock_AutoAdvance_IncrementOnEachCall(t *testing.T) {
	initial := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := clock.NewFakeClock(initial)
	c.SetAutoAdvance(time.Second)

	first := c.Now()
	second := c.Now()
	third := c.Now()

	assert.Equal(t, initial, first)
	assert.Equal(t, initial.Add(time.Second), second)
	assert.Equal(t, initial.Add(2*time.Second), third)
}

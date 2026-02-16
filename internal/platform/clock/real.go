package clock

import "time"

// RealClock returns the current UTC time from the system clock.
type RealClock struct{}

// NewRealClock creates a RealClock instance.
func NewRealClock() *RealClock {
	return &RealClock{}
}

// Now returns the current time in UTC.
func (c *RealClock) Now() time.Time {
	return time.Now().UTC()
}

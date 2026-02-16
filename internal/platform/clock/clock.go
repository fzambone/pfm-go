package clock

import "time"

// Clock provides the current time. Production code uses RealClock;
// tests use FakeClock for deterministic behavior.
type Clock interface {
	Now() time.Time
}

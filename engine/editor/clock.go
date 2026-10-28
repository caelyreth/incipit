package editor

import "time"

// Clock supplies the wall-clock time used by transient editor state. Tests
// inject a deterministic clock instead of reaching for time.Now.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the host clock.
type SystemClock struct{}

// Now returns the current host time.
func (SystemClock) Now() time.Time {
	return time.Now()
}

func clockOrSystem(clock Clock) Clock {
	if clock == nil {
		return SystemClock{}
	}
	return clock
}

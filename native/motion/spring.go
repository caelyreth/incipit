package motion

import (
	"math"
	"time"
)

// A spring stops once it is this close to its target, in DIPs, and moving
// slower than this, in DIPs per second.
const (
	positionEpsilon = float32(0.25)
	velocityEpsilon = float32(8)
)

// Spring is a critically configured exponential spring that preserves velocity
// across retargeting. Time is always supplied by the caller.
type Spring struct {
	value    float32
	velocity float32
	target   float32
	updated  time.Time
	unit     bool
}

func NewSpring(value float32, now time.Time) Spring {
	return Spring{value: value, target: value, updated: now}
}

// NewUnitSpring follows a dimensionless value, such as opacity or zoom. Its
// stopping thresholds are scaled to that domain rather than to DIPs.
func NewUnitSpring(value float32, now time.Time) Spring {
	return Spring{value: value, target: value, updated: now, unit: true}
}

func (s *Spring) Step(target float32, now time.Time, frequency float32) float32 {
	dt := now.Sub(s.updated).Seconds()
	if dt < 0 {
		dt = 0
	}
	if dt > 0 {
		displacement := s.value - s.target
		travel := (frequency*displacement + s.velocity) * float32(dt)
		decay := float32(math.Exp(float64(-frequency * float32(dt))))
		s.value = (displacement+travel)*decay + s.target
		s.velocity = (-frequency*travel + s.velocity) * decay
		s.updated = now
		if !s.Moving() {
			s.value = s.target
			s.velocity = 0
		}
	}
	s.target = target
	return s.value
}

func (s Spring) Value() float32  { return s.value }
func (s Spring) Target() float32 { return s.target }

func (s Spring) MovingTo(target float32) bool {
	position, velocity := positionEpsilon, velocityEpsilon
	if s.unit {
		position /= 100
		velocity /= 100
	}
	return abs(s.value-target) > position || abs(s.velocity) > velocity
}

func (s Spring) Moving() bool {
	return s.MovingTo(s.target)
}

func abs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

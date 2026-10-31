// Package scrollfx adds physical edge response to scroll surfaces.
package scrollfx

import (
	"math"
	"time"
)

const (
	// maxSpeed keeps a rapid stream of input from making the recoil
	// physically implausible.
	maxSpeed = float32(1200)
	// inputHold is the quiet time that ends a gesture. Until it passes, the
	// edge follows the touchpad and never springs back between its events.
	inputHold = 90 * time.Millisecond
	// releaseSpeed keeps the hand's last motion without letting it carry the
	// surface visibly farther out after the gesture stops.
	releaseSpeed = float32(120)
	substep      = float32(1) / 240
)

// Profile is how one scroll surface answers being pulled past its edge.
// Resistance below 1 makes travel progressively harder; Stiffness and
// Damping are the return spring's physical terms.
type Profile struct {
	Resistance float32
	Stiffness  float32
	Damping    float32
}

// DefaultProfile is a quick return with no second visible swing.
var DefaultProfile = Profile{Resistance: 0.55, Stiffness: 260, Damping: 32}

// Elastic is one axis of a surface pulled beyond its content edge.
type Elastic struct {
	offset   float32
	velocity float32
	spring   Profile
	updated  time.Time
	last     time.Time
	held     bool
	coasting bool
}

// InputPhase is how scroll input relates to a direct gesture.
type InputPhase uint8

const (
	// Notch is a mouse wheel step: discrete input with no held finger, so
	// its return begins at once.
	Notch InputPhase = iota + 1
	// Direct is a precise touchpad gesture still under the fingers.
	Direct
	// Momentum is the inertia after a precise gesture ended.
	Momentum
)

// Offset is the current visual displacement from the edge.
func (e Elastic) Offset() float32 { return e.offset }

func (e Elastic) moving() bool {
	return e.held || abs(e.offset) > 0.05 || abs(e.velocity) > 2
}

// Add pulls the axis by delta DIPs, resisting as it moves farther out. A pull
// toward the edge follows the input directly and never crosses through it.
func (e *Elastic) Add(
	delta, limit float32,
	now time.Time,
	phase InputPhase,
	profile Profile,
) {
	if e == nil {
		return
	}
	if limit <= 0 {
		e.Reset()
		return
	}
	e.advance(now)
	if delta == 0 {
		return
	}
	e.spring = profile.normalized()
	if phase == Momentum && e.coasting {
		// Momentum after the first release keeps driving an ordinary
		// scroll, but at an edge it must not keep restretching the band.
		return
	}
	hold := phase == Direct
	wasHeld := e.held && now.Sub(e.last) < inputHold
	if e.offset*delta < 0 {
		next := e.offset + delta
		if e.offset*next < 0 {
			next = 0
		}
		e.offset = clamp(next, -limit, limit)
		e.velocity = 0
		e.updated = now
		e.last = now
		e.held = hold
		e.coasting = phase == Momentum
		return
	}
	room := max(0, limit-abs(e.offset))
	moved := rubber(delta, room, e.spring.Resistance)
	e.offset = clamp(e.offset+moved, -limit, limit)
	if hold && wasHeld {
		// Keep only a modest part of the hand's current speed. The spring
		// does not run until this gesture pauses.
		if dt := float32(now.Sub(e.last).Seconds()); dt > 0 && dt < 0.1 {
			e.velocity = clamp(moved/dt, -releaseSpeed, releaseSpeed)
		}
	} else {
		e.velocity = 0
	}
	e.updated = now
	e.last = now
	e.held = hold
	e.coasting = phase == Momentum
}

// Step advances the return spring and reports the displacement and whether
// another frame is needed. With motion disabled, it returns to the edge at
// once.
func (e *Elastic) Step(now time.Time, animate bool) (float32, bool) {
	if e == nil {
		return 0, false
	}
	if !animate {
		e.Reset()
		return 0, false
	}
	if e.held && now.Sub(e.last) < inputHold {
		e.updated = now
		return e.offset, abs(e.offset) > 0
	}
	e.advance(now)
	if !e.moving() {
		e.offset, e.velocity, e.updated = 0, 0, now
		return 0, false
	}
	return e.offset, true
}

// Reset puts the axis back at its edge.
func (e *Elastic) Reset() {
	if e == nil {
		return
	}
	*e = Elastic{}
}

func (e *Elastic) advance(now time.Time) {
	if e.updated.IsZero() {
		e.updated = now
		return
	}
	if e.held {
		if now.Sub(e.last) < inputHold {
			e.updated = now
			return
		}
		e.held = false
		e.updated = now
		e.velocity = clamp(e.velocity, -releaseSpeed, releaseSpeed)
	}
	dt := float32(now.Sub(e.updated).Seconds())
	if dt <= 0 {
		return
	}
	// A surface away from the screen has already had its recoil. Forget
	// that elapsed time instead of integrating it in one unstable step.
	if dt > float32((250 * time.Millisecond).Seconds()) {
		e.Reset()
		e.updated = now
		return
	}
	for dt > 0 {
		step := min(dt, substep)
		acceleration := -e.spring.Stiffness*e.offset - e.spring.Damping*e.velocity
		e.velocity = clamp(e.velocity+acceleration*step, -maxSpeed, maxSpeed)
		e.offset += e.velocity * step
		dt -= step
	}
	e.updated = now
}

// rubber maps unbounded pull to bounded travel. It follows the usual
// diminishing-return curve: the first DIPs move nearly as far as the input,
// while the edge becomes progressively harder to leave.
func rubber(delta, room, resistance float32) float32 {
	if delta == 0 || room <= 0 {
		return 0
	}
	direction := float32(1)
	if delta < 0 {
		direction = -1
	}
	distance := abs(delta)
	return direction * distance * room / (room/resistance + distance)
}

func (p Profile) normalized() Profile {
	if p.Resistance <= 0 {
		p.Resistance = DefaultProfile.Resistance
	}
	if p.Stiffness <= 0 {
		p.Stiffness = DefaultProfile.Stiffness
	}
	if p.Damping <= 0 {
		p.Damping = DefaultProfile.Damping
	}
	return p
}

func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}

func abs(value float32) float32 {
	return float32(math.Abs(float64(value)))
}

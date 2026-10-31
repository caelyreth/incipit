// Package scrollfx adds physical edge response to scroll surfaces.
package scrollfx

import (
	"math"
	"time"
)

const (
	// inputHold is the quiet time that ends a touchpad gesture. Until it
	// passes, the edge stays under the fingers and never springs back
	// between their events.
	inputHold = 90 * time.Millisecond
	// releaseSpeed bounds the hand's motion carried into the return, so a
	// last flick does not throw the edge visibly farther out.
	releaseSpeed = float32(480)
	// maxSpeed keeps a burst of wheel notches physically plausible.
	maxSpeed = float32(1600)
	// notchKick and momentumKick turn a wheel notch and a touchpad's
	// inertia into a push on the edge. A notch of d DIPs swings it out
	// about kick/e·Resistance·d before it returns.
	notchKick    = float32(1.5)
	momentumKick = float32(2.5)
	// maxStep bounds one frame's time, so a stalled frame slows the return
	// instead of jumping it.
	maxStep  = float32(1) / 20
	restStep = float32(1) / 60
	substep  = float32(1) / 240
)

// Profile is how one scroll surface answers being pulled past its edge.
// Resistance is how far the edge first moves per DIP of pull, falling
// toward none at the limit. Response is the return spring's period in
// seconds and DampingRatio its damping: 1 settles without crossing back,
// a little below it settles sooner with an overshoot too small to see.
type Profile struct {
	Resistance   float32
	Response     float32
	DampingRatio float32
}

// DefaultProfile is a soft return with no second visible swing.
var DefaultProfile = Profile{Resistance: 0.5, Response: 0.42, DampingRatio: 0.88}

func (p Profile) normalized() Profile {
	if p.Resistance <= 0 {
		p.Resistance = DefaultProfile.Resistance
	}
	if p.Response <= 0 {
		p.Response = DefaultProfile.Response
	}
	if p.DampingRatio <= 0 {
		p.DampingRatio = DefaultProfile.DampingRatio
	}
	return p
}

// omega is the spring's natural frequency.
func (p Profile) omega() float32 { return 2 * math.Pi / p.Response }

// InputPhase is how scroll input relates to a direct gesture.
type InputPhase uint8

const (
	// Notch is a mouse wheel step: discrete input with no held finger. It
	// pushes the edge, which swings out and back on its own.
	Notch InputPhase = iota + 1
	// Direct is a precise touchpad gesture still under the fingers. The
	// edge follows it exactly.
	Direct
	// Momentum is the inertia after a precise gesture ended. Only its
	// first arrival at an edge pushes it.
	Momentum
)

// Elastic is one axis of a surface pulled beyond its content edge.
//
// Input only changes its state; all time comes from Step, the frame's
// clock, since scroll events need not carry one. A held pull is kept as
// raw travel, which the rubber curve maps to the offset, so pulling back
// retraces the same path and crossing the edge hands the rest of the move
// back to the content.
type Elastic struct {
	offset   float32
	velocity float32
	// raw is the pull under the fingers before the rubber curve.
	raw     float32
	limit   float32
	spring  Profile
	updated time.Time
	// touch is when the fingers last moved, touched whether they moved
	// since the last frame.
	touch   time.Time
	touched bool
	held    bool
	// tracked is the held offset's smoothed speed, given to the spring
	// when the fingers let go.
	tracked  float32
	previous float32
	// coasting is a gesture's inertia that has already pushed the edge.
	coasting bool
	resting  bool
}

// Offset is the current visual displacement from the edge.
func (e Elastic) Offset() float32 { return e.offset }

// Observe notes scroll input wherever it lands, so the inertia of a new
// gesture can push the edge even when the gesture began inside the content.
func (e *Elastic) Observe(phase InputPhase) {
	if e != nil && phase != Momentum {
		e.coasting = false
	}
}

// Add pulls the axis by delta DIPs, positive drawing content forward from
// its edge, and returns the part of delta it did not take, which belongs
// to the content: what a gesture has left after pulling back through the
// edge, or a notch or inertia scrolling away from it.
func (e *Elastic) Add(delta, limit float32, phase InputPhase, profile Profile) float32 {
	if e == nil {
		return delta
	}
	if limit <= 0 {
		e.Reset()
		return delta
	}
	e.spring, e.limit = profile.normalized(), limit
	e.offset = clamp(e.offset, -limit, limit)
	if delta == 0 {
		return 0
	}
	outward := e.offset == 0 || e.offset*delta > 0
	switch phase {
	case Direct:
		e.coasting = false
		if !e.held {
			e.raw = e.unband(e.offset)
			e.velocity, e.tracked, e.previous = 0, 0, e.offset
		}
		e.held, e.touched = true, true
		next := e.raw + delta
		if e.raw != 0 && next*e.raw <= 0 {
			// Back through the edge: the rest scrolls the content.
			e.raw, e.offset = 0, 0
			return next
		}
		e.raw = next
		e.offset = e.band(next)
		return 0
	case Momentum:
		if e.held {
			e.release()
			e.coasting = true
			return 0
		}
		if e.coasting {
			return 0
		}
		if !outward {
			return delta
		}
		e.coasting = true
		e.push(delta, momentumKick)
		return 0
	default:
		e.coasting = false
		if e.held {
			e.release()
		}
		if !outward {
			return delta
		}
		e.push(delta, notchKick)
		return 0
	}
}

// push swings the edge out with a kick from input that does not hold it.
// Each push starts from a standstill, not against the return, so every
// notch of a run answers alike; the push weakens as the edge nears its
// limit and never crosses it.
func (e *Elastic) push(delta, kick float32) {
	room := max(0, 1-abs(e.offset)/e.limit)
	distance := min(abs(delta), e.limit)
	speed := kick * e.spring.Resistance * e.spring.omega() * distance * room * room
	if e.velocity*delta < 0 {
		e.velocity = 0
	}
	e.velocity = clamp(e.velocity+sign(delta)*speed, -maxSpeed, maxSpeed)
}

// release lets the fingers go, handing their speed to the spring.
func (e *Elastic) release() {
	e.held, e.touched, e.raw = false, false, 0
	e.velocity = clamp(e.tracked, -releaseSpeed, releaseSpeed)
}

// Step advances the axis to now and reports the displacement and whether
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
	dt := float32(0)
	if !e.updated.IsZero() && now.After(e.updated) {
		dt = min(float32(now.Sub(e.updated).Seconds()), maxStep)
		if e.resting {
			// Woken from rest, the push came within the last frame.
			dt = min(dt, restStep)
		}
	}
	last := e.updated
	if last.IsZero() || now.After(last) {
		e.updated = now
	}
	e.resting = false
	if e.touched {
		// The fingers moved since the last frame, which bounds when.
		e.touched = false
		e.touch = now
		if !last.IsZero() && last.Before(now) {
			e.touch = last
		}
	}
	if e.held {
		if now.Sub(e.touch) < inputHold {
			if dt > 0 {
				speed := (e.offset - e.previous) / dt
				e.tracked += (speed - e.tracked) / 2
			}
			e.previous = e.offset
			return e.offset, true
		}
		e.release()
	}
	if e.offset != 0 || e.velocity != 0 {
		e.integrate(dt)
	}
	if abs(e.offset) <= 0.05 && abs(e.velocity) <= 2 {
		e.offset, e.velocity, e.resting = 0, 0, true
		return 0, false
	}
	return e.offset, true
}

func (e *Elastic) integrate(dt float32) {
	e.spring = e.spring.normalized()
	omega := e.spring.omega()
	stiffness := omega * omega
	damping := 2 * e.spring.DampingRatio * omega
	for dt > 0 {
		step := min(dt, substep)
		// Semi-implicit Euler: stable for these springs at this step.
		e.velocity += (-stiffness*e.offset - damping*e.velocity) * step
		e.velocity = clamp(e.velocity, -maxSpeed, maxSpeed)
		e.offset += e.velocity * step
		if abs(e.offset) > e.limit {
			e.offset = sign(e.offset) * e.limit
			if e.velocity*e.offset > 0 {
				e.velocity = 0
			}
		}
		dt -= step
	}
}

// Reset puts the axis back at its edge.
func (e *Elastic) Reset() {
	if e == nil {
		return
	}
	*e = Elastic{}
}

// band maps raw pull to the offset shown: it starts at Resistance DIPs per
// DIP and approaches the limit without reaching it.
func (e *Elastic) band(raw float32) float32 {
	r, limit := e.spring.Resistance, e.limit
	distance := abs(raw)
	return sign(raw) * limit * r * distance / (limit + r*distance)
}

// unband is band's inverse, the raw pull that shows offset.
func (e *Elastic) unband(offset float32) float32 {
	r, limit := e.spring.Resistance, e.limit
	distance := min(abs(offset), limit*0.999)
	return sign(offset) * limit * distance / (r * (limit - distance))
}

func sign(value float32) float32 {
	switch {
	case value > 0:
		return 1
	case value < 0:
		return -1
	}
	return 0
}

func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}

func abs(value float32) float32 {
	return float32(math.Abs(float64(value)))
}

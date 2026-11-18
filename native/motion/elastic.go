// Elastic adds physical edge response to scroll surfaces.
package motion

import (
	"math"
	"time"
)

const (
	// inputHold is the quiet that ends a touchpad gesture where the system
	// does not say the fingers lifted (Lift). Until it passes, the edge
	// stays under the fingers and never springs back between their events.
	inputHold = 90 * time.Millisecond
	// releaseSpeed bounds the fingers' speed carried into the return, so
	// a last flick does not throw the edge out.
	releaseSpeed = float32(1200)
	// runGap is the quiet that ends a run of input. The first event of a
	// run is taken to span notchSpan for a wheel notch, which the app
	// glides over about that long, and a frame for a touchpad, which
	// sends one a frame.
	runGap    = 100 * time.Millisecond
	notchSpan = float32(0.12)
	frameSpan = float32(1) / 60
	// sampleMin bounds how close two events count in time, so events of
	// one frame read as one move.
	sampleMin = float32(1) / 120
	// maxStep bounds one frame's time, so a stalled frame slows the return
	// instead of jumping it.
	maxStep  = float32(1) / 20
	restStep = float32(1) / 60
	substep  = float32(1) / 240
)

// Profile is how one scroll surface answers being carried past its edge.
// Resistance is how far the edge first moves per DIP the fingers pull,
// falling as it nears the band's extent. Response is the return spring's
// period in seconds and DampingRatio its damping: 1 settles without
// crossing back.
type Profile struct {
	Resistance   float32
	Response     float32
	DampingRatio float32
}

// DefaultProfile is a soft return that never swings back.
var DefaultProfile = Profile{Resistance: 0.4, Response: 0.7, DampingRatio: 1}

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

// Elastic is one axis of a surface carried past its content edge.
//
// What moves is the offset shown, a spring that returns it to the edge.
// Fingers on a touchpad hold it: their travel past the edge maps to the
// offset by a rubber band as long as the view (band), so a pull near the
// edge moves it about Resistance as far and it gives less the farther it
// goes, never reaching the band's extent; pulling back retraces the same
// path, and crossing the edge hands the rest of the move back to the
// content. Input that does not hold it, a wheel notch or a fling's
// inertia, meets it at the speed the content was moving, measured from the
// input itself, and the spring carries that speed out and back. Letting
// go hands the spring the speed the offset had. Input only changes its
// state; the frames' clock moves it (Step).
type Elastic struct {
	offset   float32
	velocity float32
	// travel is the fingers' pull past the edge while they hold it,
	// before the band.
	travel  float32
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
	// speed is the content's speed as the input moves it, per second,
	// and sampled when it last moved; phase is the last input's.
	speed   float32
	sampled time.Time
	phase   InputPhase
	// coasting is a gesture's inertia that has already met the edge.
	coasting bool
	resting  bool
}

// Offset is the current visual displacement from the edge.
func (e Elastic) Offset() float32 { return e.offset }

// Observe notes scroll input of delta at at wherever it lands: how fast it
// moves the content, so that meeting the edge carries that speed, and a
// new gesture, so that its inertia can meet the edge again. An input
// without a time is taken at the last frame's.
func (e *Elastic) Observe(delta float32, phase InputPhase, at time.Time) {
	if e == nil {
		return
	}
	if phase != Momentum {
		e.coasting = false
	}
	if delta == 0 {
		return
	}
	if at.IsZero() {
		at = e.updated
	}
	gap := float32(at.Sub(e.sampled).Seconds())
	switch {
	case e.sampled.IsZero() || gap > float32(runGap.Seconds()) || gap < 0 ||
		delta*e.speed < 0 || phase == Notch && e.phase != Notch:
		// A new run, or a turn: its speed is this move's alone.
		e.speed = delta / firstSpan(phase)
	default:
		e.speed += (delta/max(gap, sampleMin) - e.speed) / 2
	}
	e.sampled, e.phase = at, phase
}

// Lift lets the fingers go at once, as a touchpad says they lifted: an
// empty event of a direct gesture.
func (e *Elastic) Lift() {
	if e != nil && e.held {
		e.release()
	}
}

// Add carries the axis by delta DIPs, positive drawing content forward
// from its edge, and returns the part of delta it did not take, which
// belongs to the content: what a gesture has left after pulling back
// through the edge, or input going away from the edge. Input that does not
// hold the edge meets it at the speed Observe measured.
func (e *Elastic) Add(delta, limit float32, phase InputPhase, profile Profile) float32 {
	if e == nil {
		return delta
	}
	if limit <= 0 {
		e.Reset()
		return delta
	}
	e.spring, e.limit = profile.normalized(), limit
	if delta == 0 {
		return 0
	}
	outward := e.offset == 0 || e.offset*delta > 0
	switch phase {
	case Direct:
		e.coasting = false
		if !e.held {
			// The fingers take the edge where it is shown.
			e.travel = e.unband(e.offset)
			e.velocity, e.tracked, e.previous = 0, 0, e.offset
		}
		e.held, e.touched = true, true
		next := e.travel + delta
		if e.travel != 0 && next*e.travel <= 0 {
			// Back through the edge: the rest scrolls the content, and
			// the fingers hold the edge no more, so letting go throws
			// nothing past it.
			e.offset, e.travel, e.velocity, e.tracked = 0, 0, 0, 0
			e.held, e.touched = false, false
			return next
		}
		e.travel = next
		e.offset = e.band(next)
		return 0
	case Momentum:
		if e.held {
			e.release()
		}
		if !outward {
			e.inward()
			return delta
		}
		if e.coasting {
			return 0
		}
		e.coasting = true
		e.meet(delta)
		return 0
	default:
		e.coasting = false
		if e.held {
			e.release()
		}
		if !outward {
			e.inward()
			return delta
		}
		e.meet(delta)
		return 0
	}
}

// meet has the edge met by input moving delta: the spring takes the
// content's speed as it came, resisted as a pull is, unless the edge
// already goes out faster. A
// move with no speed measured toward the edge is a run's first event.
func (e *Elastic) meet(delta float32) {
	// Observe measured the speed in the content's sense, the opposite of
	// the edge's.
	speed := -e.speed
	if speed*delta <= 0 {
		speed = delta / firstSpan(e.phase)
	}
	// The edge resists the content as it does the fingers: it moves
	// Resistance as fast as what meets it.
	speed *= e.spring.Resistance
	if e.velocity*delta > 0 && abs(e.velocity) >= abs(speed) {
		return
	}
	e.velocity = speed
}

// firstSpan is how long the first event of a run of phase is taken to
// span.
func firstSpan(phase InputPhase) float32 {
	if phase == Notch {
		return notchSpan
	}
	return frameSpan
}

// inward lets input going back from the edge stop its going out: the
// spring returns from where it is.
func (e *Elastic) inward() {
	if e.velocity*e.offset > 0 {
		e.velocity = 0
	}
}

// release lets the fingers go, handing the offset's speed to the spring.
func (e *Elastic) release() {
	e.held, e.touched, e.travel = false, false, 0
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
		e.settle()
		return 0, false
	}
	dt := float32(0)
	if !e.updated.IsZero() && now.After(e.updated) {
		dt = min(float32(now.Sub(e.updated).Seconds()), maxStep)
		if e.resting {
			// Woken from rest, the input came within the last frame.
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
		// The fingers have been still since: they let go with no speed.
		e.tracked = 0
		e.release()
	}
	if e.offset != 0 || e.velocity != 0 {
		e.integrate(dt)
	}
	// It rests where the eye can no longer tell, as the page's glides do.
	if abs(e.offset) <= 0.25 && abs(e.velocity) <= 8 {
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
		e.offset += e.velocity * step
		dt -= step
	}
	// The band's extent bounds even the hardest fling.
	if e.limit > 0 && abs(e.offset) > e.limit {
		e.offset = sign(e.offset) * e.limit
		if e.velocity*e.offset > 0 {
			e.velocity = 0
		}
	}
}

// settle puts the axis at its edge, keeping what it knows of the input.
func (e *Elastic) settle() {
	e.offset, e.velocity, e.travel, e.held, e.touched, e.tracked = 0, 0, 0, false, false, 0
}

// Reset puts the axis back at its edge.
func (e *Elastic) Reset() {
	if e == nil {
		return
	}
	*e = Elastic{}
}

// band maps the fingers' travel past the edge to the offset shown: it
// starts at Resistance DIPs per DIP and approaches the band's extent, the
// limit, without reaching it.
func (e Elastic) band(travel float32) float32 {
	r, limit := e.spring.Resistance, e.limit
	if travel == 0 || limit <= 0 {
		return 0
	}
	distance := abs(travel)
	return sign(travel) * limit * r * distance / (limit + r*distance)
}

// unband is band's inverse, the travel that shows offset.
func (e Elastic) unband(offset float32) float32 {
	r, limit := e.spring.Resistance, e.limit
	if offset == 0 || limit <= 0 || r <= 0 {
		return 0
	}
	distance := min(abs(offset), limit*0.99)
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

package motion

import (
	"math"
	"time"
)

// ViewportGlide is how long a page takes to move to a new offset.
const ViewportGlide = 650 * time.Millisecond

// focusSharpness sets how front-loaded the glide is: the larger, the harder
// the rise and the longer the soft landing.
const focusSharpness = 7

// glideRate is the exponential's rate over ViewportGlide.
var glideRate = float32(focusSharpness * math.Ln2 / ViewportGlide.Seconds())

const (
	// A glide ends this close to rest, in DIPs and DIPs a second, or past
	// its deadline.
	glideSettle   = float32(0.25)
	glideStill    = float32(8)
	glideDeadline = 3 * ViewportGlide / 2
)

// Glide moves a viewport toward an offset. From rest it rises hard and
// lands softly, the way a lens pulls focus. When a new destination
// interrupts it, the new glide keeps the speed the old one had at that
// instant, so the page never lurches:
//
//	x(t) = to + (lead + carry·t)·e^(-k·t)
//
// the critically damped path through the start and the velocity handed
// over, so it lands without overshoot.
type Glide struct {
	moving          bool
	to, lead, carry float32
	started         time.Time
}

func (g *Glide) at(now time.Time) (float32, float32, bool) {
	elapsed := now.Sub(g.started)
	t := float32(elapsed.Seconds())
	decay := float32(math.Exp(float64(-glideRate * t)))
	offset := (g.lead + g.carry*t) * decay
	velocity := (g.carry - glideRate*(g.lead+g.carry*t)) * decay
	if elapsed >= glideDeadline || abs(offset) < glideSettle && abs(velocity) < glideStill && elapsed > 0 {
		return g.to, 0, false
	}
	return g.to + offset, velocity, true
}

// To glides from current to target. A glide in flight hands over from
// where it is now and at the speed it is going.
func (g *Glide) To(current, target float32, now time.Time) {
	if g.moving && g.to == target {
		return
	}
	lead, carry := current-target, float32(0)
	if g.moving {
		position, velocity, moving := g.at(now)
		lead = position - target
		if moving {
			carry = velocity + glideRate*lead
		}
	}
	*g = Glide{moving: true, to: target, lead: lead, carry: carry, started: now}
}

// Settle redirects the glide when its destination stops applying, as when
// typewriter mode turns off mid-glide: the page coasts to where its speed
// would carry it, kept within [low, high], and eases to rest there instead
// of finishing the old glide and snapping into range.
func (g *Glide) Settle(current, low, high float32, now time.Time) {
	position, velocity := current, float32(0)
	if g.moving {
		position, velocity, _ = g.at(now)
	}
	target := max(low, min(position+velocity/glideRate, high))
	if !g.moving && target == current {
		return
	}
	g.To(position, target, now)
}

// Step is the glide's position now, and whether it still moves; with
// motion off it lands at once. A glide at rest returns current.
func (g *Glide) Step(current float32, now time.Time, animate bool) (float32, bool) {
	if !g.moving {
		return current, false
	}
	position, _, moving := g.at(now)
	if !animate {
		position, moving = g.to, false
	}
	g.moving = moving
	return position, moving
}

// Moving reports a glide in flight.
func (g *Glide) Moving() bool { return g.moving }

// Cancel stops the glide where it is.
func (g *Glide) Cancel() { g.moving = false }

// Reveal is the offset that keeps a caret's line in view, and whether the
// offset changes: in typewriter mode the line sits at the exact center
// unless a scroll by hand holds it; otherwise it moves only to bring the
// line in, with a margin.
func Reveal(offset, viewport, lineY, lineH float32, typewriter, held bool) (float32, bool) {
	const margin = 8
	var target float32
	switch {
	case typewriter && !held:
		target = lineY + lineH/2 - viewport/2
	case lineY < offset+margin:
		target = lineY - margin
	case lineY+lineH > offset+viewport-margin:
		target = lineY + lineH - viewport + margin
	default:
		return offset, false
	}
	return target, target != offset
}

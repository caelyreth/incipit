package motion

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// CaretSolid is how long the caret stays fully opaque after activity. It
// is longer than the gap between keystrokes, so the caret stays solid
// while typing. CaretWake and CaretOpacity both read it: a wake at any
// other time lets breathing start between frames and the caret jump.
const CaretSolid = 240 * time.Millisecond

// CaretBreath is one full transparent-and-back breathing period.
const CaretBreath = 1050 * time.Millisecond

// CaretWake is when a caret last active at activity starts to breathe,
// and whether that is still ahead of now.
func CaretWake(activity, now time.Time) (time.Time, bool) {
	at := activity.Add(CaretSolid)
	return at, at.After(now)
}

// CaretOpacity is solid for CaretSolid after activity and then breathes
// through fully transparent and opaque over a CaretBreath cosine period.
func CaretOpacity(activity, now time.Time) float32 {
	since := now.Sub(activity)
	if since < CaretSolid {
		return 1
	}
	phase := (since - CaretSolid).Seconds() / CaretBreath.Seconds() * 2 * math.Pi
	return float32(0.5 + 0.5*math.Cos(phase))
}

// Rect glides a rectangle, each edge on its own spring.
type Rect struct {
	axes  [4]Spring
	shown bool
}

// Step glides toward target at frequency and returns where the rectangle
// is now; from nothing shown, or unanimated, it stands at target.
func (r *Rect) Step(target ui.Rect, now time.Time, frequency float32, animate bool) ui.Rect {
	values := [4]float32{target.X, target.Y, target.W, target.H}
	if !r.shown || !animate {
		for i, v := range values {
			r.axes[i] = NewSpring(v, now)
		}
		r.shown = true
		return target
	}
	var out [4]float32
	for i, v := range values {
		out[i] = r.axes[i].Step(v, now, frequency)
	}
	return ui.Rect{X: out[0], Y: out[1], W: max(0, out[2]), H: max(0, out[3])}
}

// Value is where the rectangle is drawn.
func (r *Rect) Value() ui.Rect {
	return ui.Rect{X: r.axes[0].Value(), Y: r.axes[1].Value(), W: max(0, r.axes[2].Value()), H: max(0, r.axes[3].Value())}
}

// Moving reports a rectangle still gliding.
func (r *Rect) Moving() bool {
	for i := range r.axes {
		if r.axes[i].Moving() {
			return true
		}
	}
	return false
}

// Hide lets the rectangle go: shown nowhere, it has nowhere to glide from,
// so it next stands at its place at once.
func (r *Rect) Hide() { r.shown = false }

// Shown reports a rectangle that has a place.
func (r *Rect) Shown() bool { return r.shown }

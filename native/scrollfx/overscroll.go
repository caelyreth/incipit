package scrollfx

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Axis says which ways a surface scrolls.
type Axis uint8

const (
	Vertical Axis = 1 << iota
	Horizontal
)

// Overscroll holds the physical displacement of one scroll surface.
type Overscroll struct {
	x Elastic
	y Elastic
}

// Handle takes wheel input that would leave the scroll at an edge. It also
// gives a precise touchpad move that crosses an edge in one event its exact
// remaining travel, without delaying the resistance until a later event.
func (o *Overscroll) Handle(
	event ui.InputEvent,
	state *ui.ScrollState,
	axis Axis,
	limit float32,
	profile Profile,
) bool {
	if o == nil || state == nil || event.Kind != ui.InputScroll {
		return false
	}
	dx, dy := event.DX, event.DY
	if event.Mods&ui.Shift != 0 && dx == 0 {
		dx, dy = dy, 0
	}
	handled := false
	if axis&Horizontal != 0 && dx != 0 {
		position, ok := o.pull(
			&o.x, dx, state.X, state.MaxX,
			phase(event.Precise, event.Momentum), limit, event.Time, profile,
		)
		if ok {
			state.X = position
			handled = true
		}
	}
	if axis&Vertical != 0 && dy != 0 {
		position, ok := o.pull(
			&o.y, dy, state.Y, state.MaxY,
			phase(event.Precise, event.Momentum), limit, event.Time, profile,
		)
		if ok {
			state.Y = position
			handled = true
		}
	}
	return handled
}

// Step advances both axes and reports whether another frame is needed.
func (o *Overscroll) Step(now time.Time, animate bool) (x, y float32, moving bool) {
	if o == nil {
		return 0, 0, false
	}
	x, mx := o.x.Step(now, animate)
	y, my := o.y.Step(now, animate)
	return x, y, mx || my
}

// Offset is the current visual displacement from both edges.
func (o *Overscroll) Offset() (x, y float32) {
	if o == nil {
		return 0, 0
	}
	return o.x.Offset(), o.y.Offset()
}

// Apply moves content by the current physical edge displacement.
func (o *Overscroll) Apply(c *ui.Context, content ui.Element, animate bool) {
	if o == nil || c == nil {
		return
	}
	x, y, moving := o.Step(c.Now(), animate)
	if moving {
		c.AnimationFrame()
	}
	if x != 0 || y != 0 {
		content.Translate(x, y)
	}
}

// Reset returns the surface to rest at its edge.
func (o *Overscroll) Reset() {
	if o == nil {
		return
	}
	o.x.Reset()
	o.y.Reset()
}

func (o *Overscroll) pull(
	elastic *Elastic,
	delta, position, maximum float32,
	phase InputPhase,
	limit float32,
	now time.Time,
	profile Profile,
) (float32, bool) {
	if delta == 0 {
		return position, false
	}
	if elastic.Offset() != 0 {
		elastic.Add(-delta, limit, now, phase, profile)
		return position, true
	}
	if delta > 0 {
		switch {
		case position >= maximum-0.5:
			elastic.Add(-delta, limit, now, phase, profile)
			return position, true
		case phase != Notch && position+delta > maximum:
			elastic.Add(-(position + delta - maximum), limit, now, phase, profile)
			return maximum, true
		}
		return position, false
	}
	switch {
	case position <= 0.5:
		elastic.Add(-delta, limit, now, phase, profile)
		return position, true
	case phase != Notch && position+delta < 0:
		elastic.Add(-(position + delta), limit, now, phase, profile)
		return 0, true
	}
	return position, false
}

func phase(precise, momentum bool) InputPhase {
	if momentum {
		return Momentum
	}
	if precise {
		return Direct
	}
	return Notch
}

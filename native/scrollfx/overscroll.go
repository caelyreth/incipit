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

// Handle takes scroll input the surface cannot use at an edge, and a
// touchpad move that crosses an edge in one event gets its exact remaining
// travel at once. Pulling back from a stretched edge returns to the
// content whatever passes through the edge.
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
	input := Phase(event.Precise, event.Momentum)
	o.x.Observe(input)
	o.y.Observe(input)
	handled := false
	if axis&Horizontal != 0 && dx != 0 {
		if position, ok := pull(&o.x, dx, state.X, state.MaxX, input, limit, profile); ok {
			state.X = position
			handled = true
		}
	}
	if axis&Vertical != 0 && dy != 0 {
		if position, ok := pull(&o.y, dy, state.Y, state.MaxY, input, limit, profile); ok {
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

// pull gives an axis the part of a scroll by delta that goes past an edge
// of content scrolled to position of maximum, and reports where the
// content goes and whether the input was taken. A wheel notch is left to
// the container until the content is at its edge, as the container glides
// notches there itself.
func pull(
	elastic *Elastic,
	delta, position, maximum float32,
	input InputPhase,
	limit float32,
	profile Profile,
) (float32, bool) {
	result := AxisScroll(elastic, delta, position, 0, maximum, input, limit, profile, AxisPolicy{
		EdgeTolerance: 0.5,
		Container:     true,
	})
	return result.Position, result.Handled
}

// AxisPolicy distinguishes a view that owns its scroll position from one
// where MyGo owns ordinary scrolling and only edge input is intercepted.
type AxisPolicy struct {
	EdgeTolerance float32
	Container     bool
}

// AxisResult reports the new position and whether a host scroll container
// should consume this input. An owning viewport always consumes it.
type AxisResult struct {
	Position float32
	Handled  bool
}

// AxisScroll applies one axis of input, allowing arbitrary content bounds.
// A container leaves notches and ordinary in-bounds input to MyGo; an owning
// viewport handles all phases and exact bounds itself.
func AxisScroll(
	elastic *Elastic,
	delta, position, low, high float32,
	input InputPhase,
	limit float32,
	profile Profile,
	policy AxisPolicy,
) AxisResult {
	if elastic == nil {
		return AxisResult{Position: position}
	}
	elastic.Observe(input)
	if elastic.Offset() != 0 {
		rest := -elastic.Add(-delta, limit, input, profile)
		if rest == 0 {
			return AxisResult{Position: position, Handled: true}
		}
		if policy.Container && input != Direct {
			// Scrolling away from a pushed edge: the container takes it
			// as it would, while the edge settles.
			return AxisResult{Position: position}
		}
		if policy.Container {
			return AxisResult{Position: max(low, min(position+rest, high)), Handled: true}
		}
		delta = rest
	}
	if !policy.Container {
		next := position + delta
		beyond := float32(0)
		switch {
		case next < low:
			beyond, next = low-next, low
		case next > high:
			beyond, next = high-next, high
		default:
			return AxisResult{Position: next, Handled: true}
		}
		if input == Momentum {
			beyond = -delta
		}
		elastic.Add(beyond, limit, input, profile)
		return AxisResult{Position: next, Handled: true}
	}
	beyond, edge := float32(0), position
	switch {
	case delta > 0 && position >= high-policy.EdgeTolerance:
		beyond = delta
	case delta < 0 && position <= low+policy.EdgeTolerance:
		beyond = delta
	case input == Notch:
		return AxisResult{Position: position}
	case position+delta > high:
		beyond, edge = position+delta-high, high
	case position+delta < low:
		beyond, edge = position+delta-low, low
	default:
		return AxisResult{Position: position}
	}
	push := beyond
	if input == Momentum {
		// Inertia's push is measured by its speed, the whole event.
		push = delta
	}
	elastic.Add(-push, limit, input, profile)
	return AxisResult{Position: edge, Handled: true}
}

// Phase converts native scroll flags to the shared edge-physics input phase.
func Phase(precise, momentum bool) InputPhase {
	if momentum {
		return Momentum
	}
	if precise {
		return Direct
	}
	return Notch
}

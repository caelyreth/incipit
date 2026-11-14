package components

import (
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// OverscrollStyle is one surface's edge behavior. Its rubber band reaches
// Share of the viewport, at least Minimum DIPs at the default UI size: a
// pull near the edge moves it about Resistance as far, giving less as it
// nears that extent. Response and DampingRatio are its return spring, as
// scrollfx.Profile describes them.
type OverscrollStyle struct {
	Share        float32
	Minimum      float32
	Resistance   float32
	Response     float32
	DampingRatio float32
}

// Overscroll profiles from the roomiest reading surface to controls. A
// band reaches a share of its view, so the edge gives the same at any
// window size; the smaller a surface, the shorter and firmer its band and
// the quicker its return. Each return is a loose, critically damped spring
// that eases home without swinging past the edge, and each edge meets
// what arrives at it as it resists a pull, so a slow scroll barely moves
// it and a fling carries it out.
var (
	OverscrollEditor = OverscrollStyle{
		Share: 0.35, Minimum: 120,
		Resistance: 0.4, Response: 0.55, DampingRatio: 1,
	}
	OverscrollPanel = OverscrollStyle{
		Share: 0.3, Minimum: 100,
		Resistance: 0.36, Response: 0.5, DampingRatio: 1,
	}
	OverscrollList = OverscrollStyle{
		Share: 0.25, Minimum: 80,
		Resistance: 0.32, Response: 0.46, DampingRatio: 1,
	}
	OverscrollCompact = OverscrollStyle{
		Share: 0.2, Minimum: 40,
		Resistance: 0.28, Response: 0.42, DampingRatio: 1,
	}
)

// Limit is the extent of this style's rubber band over a viewport.
func (s OverscrollStyle) Limit(viewport, uiFontSize float32) float32 {
	if uiFontSize <= 0 {
		uiFontSize = theme.DefaultUIFontSize
	}
	return max(s.Minimum*uiFontSize/theme.DefaultUIFontSize, viewport*s.Share)
}

// Scroll owns an axis's position and applies this style at its bounds:
// the event's delta along the axis, at its time.
func (s OverscrollStyle) Scroll(
	elastic *scrollfx.Elastic,
	position, delta, low, high, viewport, uiFontSize float32,
	event ui.InputEvent,
) float32 {
	phase := scrollfx.Phase(event.Precise, event.Momentum)
	if phase == scrollfx.Direct && event.DX == 0 && event.DY == 0 {
		// An empty direct event: the fingers lifted.
		elastic.Lift()
		return position
	}
	return scrollfx.AxisScroll(elastic, delta, position, low, high,
		phase, event.Time,
		s.Limit(viewport, uiFontSize), s.profile(), scrollfx.AxisPolicy{}).Position
}

// Handle takes edge input for one scroll surface with this style.
func (s OverscrollStyle) Handle(
	over *scrollfx.Overscroll,
	event ui.InputEvent,
	state *ui.ScrollState,
	axis scrollfx.Axis,
	viewport, uiFontSize float32,
) bool {
	return over.Handle(
		event,
		state,
		axis,
		s.Limit(viewport, uiFontSize),
		s.profile(),
	)
}

func (s OverscrollStyle) profile() scrollfx.Profile {
	return scrollfx.Profile{
		Resistance:   s.Resistance,
		Response:     s.Response,
		DampingRatio: s.DampingRatio,
	}
}

// BindOverscroll gives a scroll container its edge physics. The handler only
// takes input the container cannot use; everything else falls through to the
// normal scroll path.
func BindOverscroll(
	scroll ui.Element,
	state *ui.ScrollState,
	over *scrollfx.Overscroll,
	axis scrollfx.Axis,
	style OverscrollStyle,
	viewport, uiFontSize float32,
) ui.Element {
	return scroll.HandleInput(func(event ui.InputEvent) bool {
		return style.Handle(over, event, state, axis, viewport, uiFontSize)
	})
}

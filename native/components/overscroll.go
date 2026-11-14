package components

import (
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// OverscrollStyle is one surface's edge behavior. Distance and Minimum are
// DIPs at the default UI size; ViewportShare bounds travel on small panels.
// Resistance, Response and DampingRatio are its spring, as scrollfx.Profile
// describes them.
type OverscrollStyle struct {
	Distance      float32
	Minimum       float32
	ViewportShare float32
	Resistance    float32
	Response      float32
	DampingRatio  float32
}

// Overscroll profiles from the roomiest reading surface to controls. The
// smaller a surface, the shorter and firmer its give and the quicker its
// return; each meets its edge at the speed the content came, so a slow
// scroll barely moves it and a fling carries it out, and none swings back
// past its edge: every return is critically damped.
var (
	OverscrollEditor = OverscrollStyle{
		Distance: theme.DefaultOverscroll, Minimum: 20, ViewportShare: 0.18,
		Resistance: 0.55, Response: 0.5, DampingRatio: 1,
	}
	OverscrollPanel = OverscrollStyle{
		Distance: 64, Minimum: 18, ViewportShare: 0.15,
		Resistance: 0.5, Response: 0.46, DampingRatio: 1,
	}
	OverscrollList = OverscrollStyle{
		Distance: 52, Minimum: 16, ViewportShare: 0.13,
		Resistance: 0.45, Response: 0.42, DampingRatio: 1,
	}
	OverscrollCompact = OverscrollStyle{
		Distance: 32, Minimum: 12, ViewportShare: 0.1,
		Resistance: 0.38, Response: 0.36, DampingRatio: 1,
	}
)

// Limit is how far this style may show past the edge of a viewport.
func (s OverscrollStyle) Limit(viewport, uiFontSize float32) float32 {
	if uiFontSize <= 0 {
		uiFontSize = theme.DefaultUIFontSize
	}
	scale := uiFontSize / theme.DefaultUIFontSize
	distance := s.Distance * scale
	if viewport <= 0 {
		return distance
	}
	return min(distance, max(s.Minimum*scale, viewport*s.ViewportShare))
}

// Scroll owns an axis's position and applies this style at its bounds:
// the event's delta along the axis, at its time.
func (s OverscrollStyle) Scroll(
	elastic *scrollfx.Elastic,
	position, delta, low, high, viewport, uiFontSize float32,
	event ui.InputEvent,
) float32 {
	return scrollfx.AxisScroll(elastic, delta, position, low, high,
		scrollfx.Phase(event.Precise, event.Momentum), event.Time,
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

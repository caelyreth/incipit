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
// smaller a surface, the shorter and firmer its pull and the quicker its
// return, but each keeps some give: a list still answers a wheel notch at
// its end, and none swings back a second time where it can be seen.
var (
	OverscrollEditor = OverscrollStyle{
		Distance: theme.DefaultOverscroll, Minimum: 20, ViewportShare: 0.18,
		Resistance: 0.52, Response: 0.44, DampingRatio: 0.86,
	}
	OverscrollPanel = OverscrollStyle{
		Distance: 64, Minimum: 18, ViewportShare: 0.15,
		Resistance: 0.46, Response: 0.40, DampingRatio: 0.88,
	}
	OverscrollList = OverscrollStyle{
		Distance: 52, Minimum: 16, ViewportShare: 0.13,
		Resistance: 0.40, Response: 0.36, DampingRatio: 0.9,
	}
	OverscrollCompact = OverscrollStyle{
		Distance: 32, Minimum: 12, ViewportShare: 0.1,
		Resistance: 0.32, Response: 0.30, DampingRatio: 0.92,
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

// Add pulls an elastic axis with this style and returns the part of delta
// it left to the content.
func (s OverscrollStyle) Add(
	elastic *scrollfx.Elastic,
	delta, viewport, uiFontSize float32,
	phase scrollfx.InputPhase,
) float32 {
	return elastic.Add(delta, s.Limit(viewport, uiFontSize), phase, s.profile())
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

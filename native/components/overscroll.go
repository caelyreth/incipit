package components

import (
	"time"

	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// OverscrollStyle is one surface's edge behavior. Distance and Minimum are
// DIPs at the default UI size; ViewportShare bounds travel on small panels.
type OverscrollStyle struct {
	Distance      float32
	Minimum       float32
	ViewportShare float32
	Resistance    float32
	Stiffness     float32
	Damping       float32
}

// Overscroll profiles from the roomiest reading surface to controls.
var (
	OverscrollEditor = OverscrollStyle{
		Distance: theme.DefaultOverscroll, Minimum: 18, ViewportShare: 0.18,
		Resistance: 0.55, Stiffness: 260, Damping: 32,
	}
	OverscrollPanel = OverscrollStyle{
		Distance: 62, Minimum: 18, ViewportShare: 0.15,
		Resistance: 0.42, Stiffness: 340, Damping: 38,
	}
	OverscrollList = OverscrollStyle{
		Distance: 44, Minimum: 16, ViewportShare: 0.12,
		Resistance: 0.30, Stiffness: 420, Damping: 42,
	}
	OverscrollCompact = OverscrollStyle{
		Distance: 28, Minimum: 12, ViewportShare: 0.08,
		Resistance: 0.24, Stiffness: 480, Damping: 46,
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

// Add pulls an elastic axis with this style.
func (s OverscrollStyle) Add(
	elastic *scrollfx.Elastic,
	delta, viewport, uiFontSize float32,
	now time.Time,
	phase scrollfx.InputPhase,
) {
	elastic.Add(delta, s.Limit(viewport, uiFontSize), now, phase, s.profile())
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
		Resistance: s.Resistance,
		Stiffness:  s.Stiffness,
		Damping:    s.Damping,
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

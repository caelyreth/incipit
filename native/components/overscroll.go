package components

import (
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

const minimumOverscroll = 18

// PullLimit is how far a scroll surface of the given extent may show past
// its edge: generous on a page, restrained on a compact row.
func PullLimit(viewport, uiFontSize float32) float32 {
	if uiFontSize <= 0 {
		uiFontSize = theme.DefaultUIFontSize
	}
	scale := uiFontSize / theme.DefaultUIFontSize
	return scrollfx.Limit(viewport, theme.DefaultOverscroll*scale, minimumOverscroll*scale)
}

// BindOverscroll gives a scroll container its edge physics. The handler only
// takes input the container cannot use; everything else falls through to the
// normal scroll path.
func BindOverscroll(
	scroll ui.Element,
	state *ui.ScrollState,
	over *scrollfx.Overscroll,
	axis scrollfx.Axis,
	viewport, uiFontSize float32,
) ui.Element {
	return scroll.HandleInput(func(event ui.InputEvent) bool {
		return over.Handle(event, state, axis, PullLimit(viewport, uiFontSize))
	})
}

package components

import (
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Floating builds fn's elements as what floats over a window's content,
// as its controls or a dock of actions, placed with Absolute in the
// window: in the window's overlay, so they stay over the content. Menus
// and dialogs opened later go over them.
func Floating(c *ui.Context, fn func()) { ui.Overlay(c, fn) }

// Reveal brings el into view in the scroll containers around it, with
// margin room around it, as CSS's scroll-margin: what it is revealed in
// keeps a gap from the container's edge, as it does everywhere else,
// rather than meeting it. It reveals, unseen, a box margin larger than el
// on every side, among el's children, which it may be called after.
func Reveal(c *ui.Context, el ui.Element, margin float32, opts ui.ScrollIntoViewOptions) {
	el.Children(func() {
		ui.Box(c).Key("reveal").Absolute().Top(-margin).Left(-margin).Right(-margin).Bottom(-margin).
			IgnoreInput(true).ScrollIntoView(opts)
	})
}

// RevealMargin is the room Reveal keeps around a row of a list.
func RevealMargin(c *ui.Context) float32 { return theme.Space08.DIPs(UIFontSize(c)) }

// Settled is how much room something scrolled keeps, as what it holds
// takes natural: as much at once, as it grows, and easing back on
// settle's spring as it shrinks, so the view is not cut back under what is
// still leaving and closing up, all of it jumping, but follows it.
func Settled(el ui.Element, key any, natural float32, settle time.Duration, animate bool) float32 {
	if !animate {
		return natural
	}
	held := ui.Local(el, key, func() float32 { return natural })
	motion := Spring(settle)
	if natural >= *held {
		motion = ui.Animation{Duration: 1}
	}
	*held = el.AnimateTo(key, natural, motion)
	return max(*held, natural)
}

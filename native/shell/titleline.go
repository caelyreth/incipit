package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The window has no title bar: what stands at its top shares one line
// with the traffic lights on its left and the window's controls on its
// right (controls.go), the title line, which the controls set: as far
// down from the window's top as they are in from its right, and as tall
// as they are. The canvas's columns rise into it, their heads on it, the
// heads the window's title bar: a drag on one moves the window. Where a
// head passes under the traffic lights or the controls, its content keeps
// clear of them, as far as they reach into it (clearOfChrome), so nothing
// of it hides under them as the strip goes along.

// controlsPlace is where the window's controls stood as last laid out: in
// from the window's top and right edge by inset, height tall, reaching in
// from its right edge by reach, with the room around them.
type controlsPlace struct {
	inset, height, reach float32
}

// titleLine is how far the title line stands from the window's top, and
// how tall it is: the controls', or, before they are laid out, the
// chrome's gutter's half and a head's height at the type's size k.
func (s *State) titleLine(k float32) (top, height float32) {
	top, height = theme.DefaultLayout.ChromeGutter/2, sheetHead*k
	if s.controlsAt.height > 0 {
		top, height = s.controlsAt.inset, s.controlsAt.height
	}
	return top, height
}

// clearOfChrome is how far content on the title line across b, in the
// window, keeps in from b's start and end, clear of the traffic lights and
// of the controls by the chrome's gutter's half, as far as they reach into
// it, and no further than b goes.
func (s *State) clearOfChrome(c *ui.Context, b ui.Rect) (lead, trail float32) {
	if b.W <= 0 {
		return 0, 0
	}
	gap := theme.DefaultLayout.ChromeGutter / 2
	window, _ := c.Size()
	if lights := c.TitleBar().Left; lights > 0 {
		lead = max(0, min(lights+gap-b.X, b.W))
	}
	if s.controlsAt.reach > 0 {
		trail = max(0, min(b.X+b.W-(window-s.controlsAt.reach), b.W))
	}
	return lead, trail
}

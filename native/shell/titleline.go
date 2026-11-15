package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The window has no title bar: its top is the title line, the band the
// traffic lights are centred in (titleLine), shared by everything that
// stands there, each centred on the lights as they are: the window's
// controls at its right (controls.go), the panes' heads in the page mode,
// and on the canvas the columns' heads (columnHead). The line holds
// nothing opaque of its own: the canvas's columns reach the window's top,
// what they hold beginning under the line and scrolling on under it, and
// a column's head floats over it, a compact strip as the controls are,
// shown while the pointer is on the column. The line is the window's title
// bar: a drag on it moves the window. Where a head passes under the
// traffic lights or the controls, it keeps clear of them (clearOfChrome),
// so nothing of it hides under them as the strip goes along.

// controlsPlace is where the window's controls stood as last laid out:
// reaching in from the window's right edge by reach, with the room about
// them.
type controlsPlace struct {
	reach float32
}

// titleLine is how tall the title line is from the window's top, in the
// window's points whatever the type's size: the band the traffic lights
// are centred in, or a title bar's height where there are none.
func titleLine(c *ui.Context) float32 {
	if band := c.TitleBar().Height; band > 0 {
		return band
	}
	return theme.DefaultLayout.Titlebar
}

// chromeInset is how far what stands on the title line keeps in from the
// window's edges, and from the traffic lights.
var chromeInset = theme.DefaultLayout.ChromeGutter / 2

// clearOfChrome is how far content on the title line across b, in the
// window, keeps in from b's start and end, clear of the traffic lights and
// of the controls by chromeInset, as far as they reach into it, and no
// further than b goes.
func (s *State) clearOfChrome(c *ui.Context, b ui.Rect) (lead, trail float32) {
	if b.W <= 0 {
		return 0, 0
	}
	window, _ := c.Size()
	if lights := c.TitleBar().Left; lights > 0 {
		lead = max(0, min(lights+chromeInset-b.X, b.W))
	}
	if s.controlsAt.reach > 0 {
		trail = max(0, min(b.X+b.W-(window-s.controlsAt.reach), b.W))
	}
	return lead, trail
}

// columnHead is a column's head: the title line across the column, a drag
// on it moving the window, and over it, as far shown as shown, a compact
// strip, as the controls are, at the column's start, centred on the
// traffic lights: what info shows of the column, if anything, and what
// closes it. It keeps chromeInset in from the column's ends, and lead
// and trail where the window's chrome reaches further (clearOfChrome).
func (s *State) columnHead(c *ui.Context, semantic theme.Theme, key string, lead, trail, shown float32,
	info func(), close chromeButton, animate bool) ui.Element {
	band := ui.Row(c).Key(key).Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).
		AlignItems(ui.Center).Padding(0, max(chromeInset, trail), 0, max(chromeInset, lead)).DragWindow()
	return band.Children(func() {
		if shown <= 0.001 {
			return
		}
		close.compact = true
		close.icon = components.IconClose
		groups := []components.ActionGroup{{Key: "close", Label: "Close", Actions: []func(){func() {
			close.view(c, semantic, animate)
		}}}}
		if info != nil {
			groups = append([]components.ActionGroup{{Key: "info", Label: "Sheet", Actions: []func(){info}}}, groups...)
		}
		ui.Row(c).Key("strip").MinWidth(0).Opacity(shown).Children(func() {
			components.ActionDock{Key: "head-strip", Label: "Column", Groups: groups, Compact: true, Animate: animate}.
				View(c, semantic)
		})
	})
}

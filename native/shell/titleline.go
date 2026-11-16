package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The window has no title bar: its top is the title line, the band the
// traffic lights are centred in (titleLine), shared by everything that
// stands there, each centred on the lights as they are: the desk's
// columns' heads (columnHead). The line holds nothing opaque of its own:
// the desk's columns reach the window's top,
// what they hold beginning under the line and scrolling on under it, and
// a column's head floats over it, a strip of buttons, shown while the
// pointer is on the column. The line is the window's title
// bar: a drag on it moves the window. Where a head passes under the
// traffic lights, it keeps clear of them (clearOfLights), so nothing of it
// hides under them as the strip goes along.

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

// clearOfLights is how far content on the title line across b, in the
// window, keeps in from b's start, clear of the traffic lights by
// chromeInset as far as they reach into it, and no further than b goes.
func clearOfLights(c *ui.Context, b ui.Rect) float32 {
	if lights := c.TitleBar().Left; lights > 0 && b.W > 0 {
		return max(0, min(lights+chromeInset-b.X, b.W))
	}
	return 0
}

// columnHead is a column's head: the title line across the column, a drag
// on it moving the window, and over it, as far shown as shown, a strip of
// buttons at the column's start,
// centred on the traffic lights: what info shows of the column, if anything, and what
// closes it. It keeps chromeInset in from the column's ends, and lead
// where the traffic lights reach further (clearOfLights).
func (s *State) columnHead(c *ui.Context, semantic theme.Theme, key string, lead, shown float32,
	info func(), close chromeButton, animate bool) ui.Element {
	band := ui.Row(c).Key(key).Absolute().Top(0).Left(0).Right(0).Height(titleLine(c)).
		AlignItems(ui.Center).Padding(0, chromeInset, 0, max(chromeInset, lead)).DragWindow()
	return band.Children(func() {
		if shown <= 0.001 {
			return
		}
		close.icon = components.IconClose
		groups := []components.ActionGroup{{Key: "close", Label: "Close", Actions: []func(){func() {
			close.view(c, semantic, animate)
		}}}}
		if info != nil {
			groups = append([]components.ActionGroup{{Key: "info", Label: "Sheet", Actions: []func(){info}}}, groups...)
		}
		ui.Row(c).Key("strip").MinWidth(0).Opacity(shown).Children(func() {
			components.ActionDock{Key: "head-strip", Label: "Column", Groups: groups, Bare: 1, Joined: true, Animate: animate}.
				View(c, semantic)
		})
	})
}

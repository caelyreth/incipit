package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Last places a sheet or a group after all the others.
const Last = 1 << 30

// createArea makes a thing where it will show: a dashed outline the size
// of a row, at the end of a list or, called up, at its head. It stands in
// for a button that makes things, so what is made comes in beside the
// pointer that made it.
func (s *State) createArea(c *ui.Context, semantic theme.Theme, key, label string, animate bool, action func()) ui.Element {
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	area := ui.ButtonBase(c.Key(key)).Label(label).
		WidthPercent(100).Height(metrics.Height).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).FocusRing(false)
	hover := components.HoverAmount(area, area.Hovered() || area.FocusVisible(), animate)
	press := float32(0)
	if area.Pressed() {
		press = 1
	}
	if animate {
		press = components.Follow(area, "press", press, components.MotionImmediate)
		area.Scale(1-0.02*press, 1-0.02*press)
	}
	// At rest the outline only hints; under the pointer it takes the
	// accent, as what it makes would.
	line := components.Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
	area.Background(components.Blend(ui.Transparent, semantic.Accent.Muted, 0.6*hover)).
		Draw(func(p *ui.Painter, r ui.Rect) { components.DashedOutline(p, r, metrics.Radius, 1, line) })
	ink := components.Blend(semantic.Text.Muted, semantic.Accent.Foreground, hover)
	area.Children(func() {
		ui.Icon(c, components.IconPlus.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(ink).Shrink(0)
		components.Label(c, label, theme.TextSmall, ink).SingleLine().MinWidth(0)
	})
	if area.Clicked() {
		action()
	}
	return area
}

// gap is a place in a list where a create area is called up: the pointer
// resting by it a moment opens the area there, and away a moment closes
// it. A gap is numbered by the row it comes before. Once used, it waits
// for the pointer to leave before it opens again, so the row it made is
// not pushed aside.
type gap struct {
	open  bool
	at    int
	spent bool
	// over is whether the pointer was on the open area last frame, which
	// keeps it open.
	over bool
	// want is where the pointer has rested since since, -1 away.
	want  int
	since time.Time
}

// follow reports whether the gap is open, and where, at being the gap the
// pointer is by, -1 for none.
func (g *gap) follow(c *ui.Context, at int) (int, bool) {
	if g.over {
		at = g.at
	}
	g.over = false
	current := -1
	if g.open {
		current = g.at
	}
	if g.spent && at >= 0 || !g.spent && at == current {
		g.since = time.Time{}
		return g.at, g.open
	}
	// Away, it waits less to close, or to be ready again once used, than
	// it waits to open by a row: the pointer crossing the space between
	// rows as the area folds is not away.
	wait := dwellOpen
	if at < 0 {
		wait = dwellClose
	}
	if !g.wait(c, at, wait) {
		return g.at, g.open
	}
	if g.spent {
		g.spent = false
		return g.at, g.open
	}
	g.open, g.at = at >= 0, max(at, 0)
	return g.at, g.open
}

// wait reports whether the pointer has been at at for d.
func (g *gap) wait(c *ui.Context, at int, d time.Duration) bool {
	now := c.Now()
	if g.since.IsZero() || g.want != at {
		g.want, g.since = at, now
	}
	if left := d - now.Sub(g.since); left > 0 {
		c.After(left)
		return false
	}
	g.since = time.Time{}
	return true
}

// hold keeps the gap open while the pointer is on its area.
func (g *gap) hold(area ui.Element) {
	g.over = area.Hovered()
}

// use closes the gap as what it made comes in.
func (g *gap) use() {
	g.open, g.spent, g.since = false, true, time.Time{}
}

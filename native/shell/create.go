package shell

import (
	"strings"
	"time"
	"unicode"

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
	return s.createAreaIcon(c, semantic, key, label, components.IconPlus, animate, action)
}

func (s *State) createAreaIcon(
	c *ui.Context,
	semantic theme.Theme,
	key, label string,
	icon components.IconName,
	animate bool,
	action func(),
) ui.Element {
	return s.createAreaReveal(c, semantic, key, label, icon, 1, 1, 0, animate, action)
}

func (s *State) createAreaReveal(
	c *ui.Context,
	semantic theme.Theme,
	key, label string,
	icon components.IconName,
	contentReveal, labelReveal, labelShift float32,
	animate bool,
	action func(),
) ui.Element {
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
		ui.Icon(c, icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(ink).Shrink(0).Opacity(contentReveal)
		components.Label(c, creatorLabel(label), theme.TextSmall, ink).SingleLine().MinWidth(0).
			Opacity(labelReveal).Translate(labelShift, 0)
	})
	if area.Clicked() {
		action()
	}
	return area
}

func creatorLabel(label string) string {
	visible := strings.TrimPrefix(label, "New ")
	if runes := []rune(visible); len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		visible = string(runes)
	}
	return visible
}

type createPairDwell struct {
	since time.Time
	open  bool
}

type createPairDwellKey struct{}

// createPair calls a chapter divider up beside a sheet creator as the
// pointer rests on it. Both creators share the row while it is open.
func (s *State) createPair(
	c *ui.Context,
	semantic theme.Theme,
	key string,
	animate bool,
	sheetAction, chapterAction func(),
) ui.Element {
	scale := s.Settings.UIFontSize
	pair := ui.Row(c).Key(key + "-pair").WidthPercent(100).Shrink(0)
	dwell := *ui.Local(pair, createPairDwellKey{}, func() *createPairDwell {
		return &createPairDwell{}
	})
	now := c.Now()
	if pair.Hovered() {
		if !dwell.open {
			if dwell.since.IsZero() {
				dwell.since = now
			}
			if left := dwellOpen - now.Sub(dwell.since); left > 0 {
				c.After(left)
			} else {
				dwell.open, dwell.since = true, time.Time{}
			}
		}
	} else {
		dwell.open, dwell.since = false, time.Time{}
	}
	target := float32(0)
	if dwell.open {
		target = 1
	}
	reveal := target
	if animate {
		reveal = components.Follow(pair, "chapter-reveal", target, components.MotionNormal)
	}
	reveal = max(0, min(reveal, 1))
	gap := theme.Space06.DIPs(scale) * reveal
	chapterWidth := max(0, (pair.LayoutBounds().W-gap)*0.4) * reveal
	pair.Gap(gap)
	contentReveal := min(1, reveal*2)
	labelReveal := components.Smoothstep(max(0, (reveal-0.25)/0.75))
	labelShift := (1 - labelReveal) * 4 * scale / theme.DefaultUIFontSize
	pair.Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			s.createArea(c, semantic, key, "New sheet", animate, sheetAction)
		})
		chapter := ui.Box(c).Width(chapterWidth).Shrink(0).Clip().
			Hidden(reveal <= 0.01)
		chapter.Children(func() {
			s.createAreaReveal(
				c, semantic, key+"-chapter", "New chapter", components.IconPlus,
				contentReveal, labelReveal, labelShift, animate, chapterAction,
			)
		})
	})
	return pair
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
	// quiet keeps the gap shut until then, as after a drag.
	quiet time.Time
}

// follow reports whether the gap is open, and where, at being the gap the
// pointer is by, -1 for none.
func (g *gap) follow(c *ui.Context, at int) (int, bool) {
	if now := c.Now(); now.Before(g.quiet) {
		g.close()
		c.After(g.quiet.Sub(now))
		return g.at, false
	}
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

// holdAround keeps the gap open while the pointer is on its area or within
// reach of it in list, as in the space between it and the rows around it.
func (g *gap) holdAround(area, list ui.Element, reach float32) {
	g.hold(area)
	if g.over {
		return
	}
	_, y, in := list.PointerPosition()
	bounds := area.LayoutBounds()
	if !in || bounds.H <= 0 {
		return
	}
	top := bounds.Y - list.LayoutBounds().Y
	g.over = y >= top-reach && y <= top+bounds.H+reach
}

// use closes the gap as what it made comes in.
func (g *gap) use() {
	g.open, g.spent, g.since = false, true, time.Time{}
}

// hush keeps the gap shut until then.
func (g *gap) hush(until time.Time) {
	if until.After(g.quiet) {
		g.quiet = until
	}
}

// close hides the gap at once, without waiting for the pointer to leave,
// as while a sheet is dragged over the list.
func (g *gap) close() {
	g.open, g.over, g.want, g.since = false, false, -1, time.Time{}
}

package components

import (
	"strings"
	"time"
	"unicode"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A create area makes a thing where it will show: a dashed outline the
// size of what it makes, at the end of a run of them or, called up, in a
// gap between two. It stands in for a button that makes things, so what
// is made comes in beside the pointer that made it, as the sheet list's
// rows and a table's rows and columns do.
const (
	// DwellOpen is how long the pointer rests by a gap before a create
	// area opens there, and DwellClose how long it is away before the area
	// closes.
	DwellOpen  = 350 * time.Millisecond
	DwellClose = 250 * time.Millisecond
)

// CreateArea is a dashed area that makes a thing when clicked.
type CreateArea struct {
	Key string
	// Label names what it makes, as "New sheet"; the area shows it without
	// its "New ".
	Label string
	// Icon is IconPlus when empty.
	Icon IconName
	// Reveal and LabelReveal fade the icon and the label in from 0 to 1,
	// and LabelShift slides the label from the side as it comes, for an
	// area unfolding; the zero CreateArea is wholly shown.
	Reveal, LabelReveal, LabelShift float32
	// Partial says Reveal and LabelReveal are set, not left at shown.
	Partial bool
	// Compact shows the icon alone, for an area too narrow for words.
	Compact bool
	Animate bool
	Action  func()
}

// View draws the area as wide as its parent and a control tall; the caller
// may size it otherwise.
func (a CreateArea) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	icon := a.Icon
	if icon == "" {
		icon = IconPlus
	}
	reveal, labelReveal := float32(1), float32(1)
	if a.Partial {
		reveal, labelReveal = a.Reveal, a.LabelReveal
	}
	area := ui.ButtonBase(c.Key(a.Key)).Label(a.Label).KeepFocus().
		WidthPercent(100).Height(metrics.Height).Shrink(0).
		PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).FocusRing(false)
	if a.Compact {
		area.PaddingX(0).Justify(ui.Center)
	}
	hover := HoverAmount(area, area.Hovered() || area.FocusVisible(), a.Animate)
	press := float32(0)
	if area.Pressed() {
		press = 1
	}
	if a.Animate {
		press = Follow(area, "press", press, MotionImmediate)
		area.Scale(1-0.02*press, 1-0.02*press)
	}
	// At rest the outline only hints; under the pointer it takes the
	// accent, as what it makes would.
	line := Blend(semantic.Border.Default, semantic.Accent.Foreground, hover)
	area.Background(Blend(ui.Transparent, semantic.Accent.Tint, hover)).
		Draw(func(p *ui.Painter, r ui.Rect) { DashedOutline(p, r, metrics.Radius, 1, line) })
	ink := Blend(semantic.Text.Muted, semantic.Accent.Foreground, hover)
	area.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(ink).Shrink(0).Opacity(reveal)
		if a.Compact {
			return
		}
		CenteredLabel(c, CreatorLabel(a.Label), theme.TextSmall, ink).MinWidth(0).
			Opacity(labelReveal).Translate(a.LabelShift, -UIOpticalLift(scale))
	})
	if area.Clicked() && a.Action != nil {
		a.Action()
	}
	return area
}

// CreatorLabel is what an area making label shows: "New sheet" as
// "Sheet".
func CreatorLabel(label string) string {
	visible := strings.TrimPrefix(label, "New ")
	if runes := []rune(visible); len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		visible = string(runes)
	}
	return visible
}

// CreateGap is a place in a run of things where a create area is called
// up: the pointer resting by it a moment opens the area there, and away a
// moment closes it. A gap is numbered by the thing it comes before. Once
// used, it waits for the pointer to leave before it opens again, so what
// it made is not pushed aside. Its zero value is shut.
type CreateGap struct {
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

// Open reports where the gap is and whether it is open, as last followed.
func (g *CreateGap) Open() (int, bool) { return g.at, g.open }

// Over reports whether the pointer was on the open area last frame.
func (g *CreateGap) Over() bool { return g.over }

// Clock is what a gap keeps time by: the time of the frame, and a frame
// asked for after a while. A view's context is one, and a painter.
type Clock interface {
	Now() time.Time
	After(d time.Duration)
}

// Follow reports whether the gap is open, and where, at being the gap the
// pointer is by, -1 for none.
func (g *CreateGap) Follow(c Clock, at int) (int, bool) {
	if now := c.Now(); now.Before(g.quiet) {
		g.Close()
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
	wait := DwellOpen
	if at < 0 {
		wait = DwellClose
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
func (g *CreateGap) wait(c Clock, at int, d time.Duration) bool {
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

// Hold keeps the gap open while the pointer is on its area.
func (g *CreateGap) Hold(area ui.Element) {
	g.over = area.Hovered()
}

// HoldAround keeps the gap open while the pointer is on its area or within
// reach of it in list, as in the space between it and the rows around it.
func (g *CreateGap) HoldAround(area, list ui.Element, reach float32) {
	g.Hold(area)
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

// HoldOver keeps the gap open while the pointer was over its area, as a
// view that draws the area itself tells.
func (g *CreateGap) HoldOver(over bool) {
	g.over = over
}

// Use closes the gap as what it made comes in.
func (g *CreateGap) Use() {
	g.open, g.spent, g.since = false, true, time.Time{}
}

// Hush keeps the gap shut until then.
func (g *CreateGap) Hush(until time.Time) {
	if until.After(g.quiet) {
		g.quiet = until
	}
}

// Close hides the gap at once, without waiting for the pointer to leave,
// as while a thing is dragged over the run.
func (g *CreateGap) Close() {
	g.open, g.over, g.want, g.since = false, false, -1, time.Time{}
}

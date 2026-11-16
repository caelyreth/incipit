package shell

import (
	"fmt"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The scrubber stands in for the strip's scroll bar: an overview of the
// strip at the desk's foot, at its left, a cell for each column and the
// window's span over them. It shows while the strip moves, and while the
// pointer is on it, and goes a while after the strip rests. A drag on it
// moves the strip under the span as far as it moves, and comes to rest at
// a column as a drag on the strip does; a press goes to the column under
// it. The cells only show the columns: the scrubber takes the pointer
// whole, so a drag may begin anywhere on it.

const (
	// scrubWidth is the most the scrubber takes across, and scrubHeight
	// its cells' height, at the reference type size.
	scrubWidth  = 168
	scrubHeight = 16
	// scrubLinger is how long the scrubber stays once the strip rests.
	scrubLinger = 900 * time.Millisecond
)

// scrubState is what the scrubber keeps between frames: where the strip
// was, and how many times it has moved, the last
// of which its stay counts from.
type scrubState struct {
	x     float32
	moves int
}

// deskFoot draws the strip's overview over the desk's foot, gutter in
// from its left and bottom, and, as it shows, a fade over the foot of the
// columns under it, so it reads over the pages running on there; both
// come and go together. It is drawn after the strip, as it reads the
// strip's columns as they were laid out.
func (s *State) deskFoot(c *ui.Context, semantic theme.Theme, gutter float32, animate bool) {
	spans := s.desk.spans
	if len(spans) < 2 {
		return
	}
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	last := spans[len(spans)-1]
	total := last.End()
	track := min(scrubWidth*k, float32(len(spans))*scrubHeight*k*2)
	f := track / total
	view := s.desk.viewport.W - 2*gutter
	if s.desk.strip.X != s.desk.scrub.x {
		s.desk.scrub.x = s.desk.strip.X
		s.desk.scrub.moves++
	}
	pad := 4 * k
	bar := ui.Box(c).Key("desk-scrubber").Absolute().Left(gutter).Bottom(gutter).
		Width(track+2*pad).Height(scrubHeight*k+2*pad).Radius(6*k).
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant).
		Role(ui.RoleGroup).Label("Desk overview")
	// The stay counts from the strip's last move: the value reaches the
	// moves so far scrubLinger after the last.
	rest := bar.AnimateTo("scrub-rest", float32(s.desk.scrub.moves), ui.Animation{Duration: scrubLinger})
	moving := rest != float32(s.desk.scrub.moves)
	if dx, _, ok := bar.Dragged(); ok && dx != 0 {
		s.stripAt(s.desk.strip.X + dx/f)
	}
	if _, _, ok := bar.DragEnded(); ok {
		s.settleStrip(s.desk.strip.X)
	} else if x, _, _ := bar.PointerPosition(); bar.Clicked() {
		s.scrubTo((x - pad) / f)
	}
	target := float32(0)
	if moving || bar.Hovered() || bar.Dragging() {
		target = 1
	}
	shown := target
	if animate {
		shown = components.Follow(bar, "scrub-shown", target, components.MotionNormal)
	}
	bar.Opacity(shown).ZIndex(1)
	if shown > 0.001 {
		depth := scrubHeight*k + 2*pad + 2*gutter
		ui.Box(c).Key("desk-foot-fade").Absolute().Left(0).Right(0).Bottom(0).Height(depth).
			IgnoreInput(true).Opacity(shown).
			Gradient(semantic.Surface.Background.Alpha(0), semantic.Surface.Background, 180)
	}
	bar.Children(func() {
		for i, sp := range spans {
			ui.Box(c).Key(fmt.Sprintf("scrub-%d", i)).Absolute().
				Left(pad + sp.X*f).Top(pad).Width(max(1, sp.W*f-k)).Height(scrubHeight * k).
				Radius(2 * k).Background(semantic.Element.Hover)
		}
		// The window's span over the cells.
		ui.Box(c).Key("scrub-span").Absolute().Left(pad+s.desk.strip.X*f).Top(pad).
			Width(min(view, total)*f).Height(scrubHeight*k).Radius(2*k).
			Border(1, semantic.Accent.Foreground)
	})
}

// scrubTo goes to the column at x across the strip's columns.
func (s *State) scrubTo(x float32) {
	if len(s.desk.spans) == 0 {
		return
	}
	at := stage.At(s.desk.spans, x)
	s.desk.strip.ScrollTo(stage.Rests(s.desk.spans, s.desk.strip.MaxX)[at], 0, deskMotion(true))
}

// settleStrip brings the strip to rest at at, or pinned to a column's
// start near it, as the magnet would (magnet).
func (s *State) settleStrip(at float32) {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	at = max(0, min(at, s.desk.strip.MaxX))
	if rest, ok := stage.NearRest(s.desk.spans, s.desk.strip.MaxX, at, magnetReach*k); ok {
		at = rest
	}
	s.desk.armed = false
	s.desk.strip.ScrollTo(at, 0, deskMotion(true))
}

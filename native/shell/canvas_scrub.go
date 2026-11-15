package shell

import (
	"fmt"
	"math"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The scrubber stands in for the strip's scroll bar: an overview of the
// strip at the canvas's foot, at its left, a cell for each column and the
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
// was, and how many times it has moved since the canvas opened, the last
// of which its stay counts from.
type scrubState struct {
	x     float32
	moves int
}

// scrubber draws the strip's overview over the canvas's foot, gutter in
// from its left and bottom. It is drawn after the strip, as it reads the
// strip's columns as they were laid out.
func (s *State) scrubber(c *ui.Context, semantic theme.Theme, gutter float32, animate bool) {
	rests := s.canvas.rests
	if len(rests) < 2 || s.canvas.width <= 0 {
		return
	}
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	total := rests[len(rests)-1] + s.canvas.width
	track := min(scrubWidth*k, float32(len(rests))*scrubHeight*k*2)
	f := track / total
	view := s.canvas.viewport.W - 2*gutter
	if s.canvas.strip.X != s.canvas.scrub.x {
		s.canvas.scrub.x = s.canvas.strip.X
		s.canvas.scrub.moves++
	}
	pad := 4 * k
	bar := ui.Box(c).Key("canvas-scrubber").Absolute().Left(gutter).Bottom(gutter).
		Width(track+2*pad).Height(scrubHeight*k+2*pad).Radius(6*k).
		Background(semantic.Surface.Panel).Border(1, semantic.Border.Variant).
		Role(ui.RoleGroup).Label("Canvas overview")
	// The stay counts from the strip's last move: the value reaches the
	// moves so far scrubLinger after the last.
	rest := bar.AnimateTo("scrub-rest", float32(s.canvas.scrub.moves), ui.Animation{Duration: scrubLinger})
	moving := rest != float32(s.canvas.scrub.moves)
	if dx, _, ok := bar.Dragged(); ok && dx != 0 {
		s.canvas.strip.X = max(0, min(s.canvas.strip.X+dx/f, total-view))
	}
	if _, _, ok := bar.DragEnded(); ok {
		s.settleStrip(s.canvas.strip.X)
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
	bar.Opacity(shown)
	bar.Children(func() {
		for i, at := range rests {
			ui.Box(c).Key(fmt.Sprintf("scrub-%d", i)).Absolute().
				Left(pad + at*f).Top(pad).Width(max(1, s.canvas.width*f-k)).Height(scrubHeight * k).
				Radius(2 * k).Background(semantic.Element.Hover)
		}
		// The window's span over the cells.
		ui.Box(c).Key("scrub-span").Absolute().Left(pad+s.canvas.strip.X*f).Top(pad).
			Width(min(view, total)*f).Height(scrubHeight*k).Radius(2*k).
			Border(1, semantic.Accent.Foreground)
	})
}

// scrubTo goes to the column at x across the strip's columns.
func (s *State) scrubTo(x float32) {
	for i := len(s.canvas.rests) - 1; i >= 0; i-- {
		if at := s.canvas.rests[i]; x >= at || i == 0 {
			s.canvas.strip.ScrollTo(at, 0, components.Spring(components.MotionScene))
			return
		}
	}
}

// settleStrip brings the strip to rest at the column whose start is
// nearest at, on the spring a column comes into view on.
func (s *State) settleStrip(at float32) {
	if len(s.canvas.rests) == 0 {
		return
	}
	rest := s.canvas.rests[0]
	for _, x := range s.canvas.rests {
		if math.Abs(float64(x-at)) < math.Abs(float64(rest-at)) {
			rest = x
		}
	}
	s.canvas.strip.ScrollTo(max(0, rest), 0, components.Spring(components.MotionScene))
}

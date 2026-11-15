package shell

import (
	"math"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The canvas is the notebook laid out as one flow, where the page mode
// shows one place and one sheet at a time: a strip of columns across the
// window, each as tall as it, that scrolls sideways without end. The first
// column is the map, every group a region of its sheets' cards; after it
// stands a column for each open sheet, written in where it stands, as many
// as are open. A touchpad's swipe, Shift and the wheel, or a drag on the
// strip moves along it, and it comes to rest with a column's start at its
// own; the sheet in front is always brought into view. There is no zoom:
// the flow is how the canvas is gone through.
//
// The map is canvas_map.go, the sheets' columns canvas_sheets.go.

const (
	// flingCarry is how far a drag let go carries the strip on, in seconds
	// of the speed it was let go at.
	flingCarry = float32(0.25)
	// columnGap is the room between the strip's columns at the reference
	// type size.
	columnGap = 24
)

// Mode is how the window lays the notebook out: the page beside its
// panes, or the canvas in their place. It is the window's top choice, as
// the panes are what the page mode shows.
type Mode uint8

const (
	ModePage Mode = iota
	ModeCanvas
)

type canvasState struct {
	// strip is where the strip is scrolled to; viewport is its window,
	// inner the width it shows its columns in, and rests where it rests at
	// each column, as last laid out.
	strip    ui.ScrollState
	viewport ui.Rect
	inner    float32
	rests    []float32
	// front is the sheet in front as the frame began, and follow whether
	// it came in front since the last, so the strip goes to it; entered is
	// the first frame since the canvas opened, which stands the sheet in
	// front in the middle at once.
	front   string
	follow  bool
	entered bool
	// reveal is a sheet to bring into view, as one opened from the map
	// that was open already.
	reveal string
}

// Mode is the window's mode; the canvas needs a library to lay out.
func (s *State) Mode() Mode {
	if s == nil || s.Library == nil {
		return ModePage
	}
	return s.mode
}

// CanvasOpen reports whether the canvas shows in place of the page.
func (s *State) CanvasOpen() bool { return s.Mode() == ModeCanvas }

// SetMode lays the notebook out as m: the canvas with the sheet in front
// where the page stood, or the page with its panes as they were.
func (s *State) SetMode(m Mode) {
	if s == nil || s.mode == m {
		return
	}
	s.mode = m
	s.canvas.entered = m == ModeCanvas
}

// ToggleCanvas goes from the page to the canvas, or back.
func (s *State) ToggleCanvas() {
	if s.CanvasOpen() {
		s.SetMode(ModePage)
		return
	}
	s.SetMode(ModeCanvas)
}

// closeCanvas goes back to the page, as choosing panes does.
func (s *State) closeCanvas() { s.SetMode(ModePage) }

// canvasView draws the canvas: the strip, with the writing actions
// floating over its foot for the sheet in front. head is the room over it
// kept for the window's traffic lights, as the panes before it close.
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	s.canvas.follow, s.canvas.front = s.canvas.front != s.ActivePage, s.ActivePage
	canvas := ui.Column(c).Key("canvas").Grow(1).MinWidth(0).Background(semantic.Surface.Background).
		Transition(fadeTransition(animate))
	canvas.Children(func() {
		s.canvasStrip(c, semantic, layout, head, animate)
		if s.ActivePage != "" {
			s.dock(c, semantic, theme.DefaultControls.Scaled(scale))
		}
		// The room over the strip moves the window, as a title bar would.
		ui.Box(c).Key("canvas-drag").Absolute().Top(0).Left(0).Right(0).Height(head).DragWindow()
	})
	// The sheet in front keeps the middle while the panes close.
	s.canvas.entered = s.canvas.entered && s.lastSidebarWidth > 0.5
	s.canvas.reveal = ""
}

// canvasStrip draws the strip under head's room: the map, then the open
// sheets' columns.
func (s *State) canvasStrip(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	gutter := layout.ContentGutter * k
	strip := ui.ScrollHorizontal(c).Key("canvas-strip").Grow(1).MinHeight(0).WidthPercent(100).
		ScrollSnap(ui.SnapProximity).TrackScroll(&s.canvas.strip)
	s.canvas.viewport = strip.LayoutBounds()
	// The columns end short of the writing actions floating under them.
	foot := gutter
	if s.ActivePage != "" {
		foot = layout.DockClearance() * k
	}
	// The columns' heads stand clear of the window's controls over them.
	top := max(head, gutter/2, s.controlsFoot)
	s.canvas.inner = s.canvas.viewport.W - 2*gutter
	strip.Padding(top, gutter, foot, gutter)
	// The columns there as the canvas opens come with it, not one by one.
	columns := animate && !s.canvas.entered
	strip.Children(func() {
		flow := ui.Row(c).Key("canvas-flow").FillHeight().AlignItems(ui.Stretch).Gap(columnGap * k)
		origin := flow.LayoutBounds().X
		s.pan(flow)
		flow.Children(func() {
			s.canvas.rests = s.canvas.rests[:0]
			s.canvasMap(c, semantic, gutter, origin, columns)
			for _, page := range s.Open {
				s.openColumn(c, semantic, page, origin, columns)
			}
			if len(s.Open) == 0 {
				s.sheetHint(c, semantic, origin, columns)
			}
		})
	})
}

// pan has a drag on el move the strip, and a drag let go carry it on to
// rest at the column nearest where it would stop. It reports a drag let
// go, whose release is no click.
func (s *State) pan(el ui.Element) bool {
	if dx, _, ok := el.Dragged(); ok && dx != 0 {
		s.canvas.strip.X = max(0, s.canvas.strip.X-dx)
	}
	vx, _, ok := el.DragEnded()
	if !ok {
		return false
	}
	if len(s.canvas.rests) == 0 {
		return true
	}
	at := s.canvas.strip.X - vx*flingCarry
	rest := s.canvas.rests[0]
	for _, x := range s.canvas.rests {
		if math.Abs(float64(x-at)) < math.Abs(float64(rest-at)) {
			rest = x
		}
	}
	s.canvas.strip.ScrollTo(max(0, rest), 0, components.Spring(components.MotionScene))
	return true
}

// column is one of the strip's columns, keyed key, w wide and as tall as
// the strip, where the strip rests with the column's start or center, as
// align says, at its own; the rest is counted for a drag to come to.
// origin is where the strip's columns start in the window.
func (s *State) column(c *ui.Context, key string, w, origin float32, align ui.Align, animate bool) ui.Element {
	col := ui.Column(c).Key(key).Width(w).Shrink(0).FillHeight().SnapAlignX(align).
		Transition(columnTransition(animate))
	if b := col.LayoutBounds(); b.W > 0 {
		x := b.X - origin
		if align == ui.Center {
			x += (b.W - s.canvas.inner) / 2
		}
		s.canvas.rests = append(s.canvas.rests, x)
	}
	return col
}

// columnTransition is how the strip's columns come and go: in from the
// side, rising to their size, out where they stood, the others closing up
// behind them, at the pace of the panes.
func columnTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{
		Duration: motionReveal, Position: true, Colors: true,
		Enter: &ui.Motion{Opacity: 0, X: enterSlide, Scale: 0.97},
		Exit:  &ui.Motion{Opacity: 0, Scale: 0.97},
	}
}

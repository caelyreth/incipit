package shell

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The canvas is the notebook laid out as one flow, where the page mode
// shows one place and one sheet at a time: a strip of columns across the
// window, each as tall as it, that scrolls sideways without end: a column
// for each open sheet, written in where it stands, and the pickers among
// them that sheets are chosen in (canvas_pickers.go).
//
// The canvas is a layer of its own over the page mode's layout, the whole
// window's, which stays laid out as it was beneath it while the canvas
// comes and goes: nothing is laid out again as the mode changes, and what
// the two share moves from one to the other (morph). The layout gone to is
// built a build ahead of what moves to it (State.held).
//
// A sheet's column is half the window, less the gap between, so two
// neighbours read as a split, or a third where a third still holds a
// sheet's page whole; a picker's is the sheet list's width. Widths change
// only as the window does, never as the strip moves or the mode changes.
// A touchpad's swipe, Shift and the wheel, or a drag on the strip moves
// along it freely, stretching past its ends; where it comes to rest by
// itself near a column's start, the column pins to the window's edge, as a
// magnet takes what comes near it (magnet). The sheet in front is always
// brought into view. There is no zoom: the flow is how the canvas is gone
// through.
//
// The sheets' columns are canvas_sheets.go, and the overview at the foot
// canvas_scrub.go.

const (
	// flingCarry is how far a drag let go carries the strip on, in seconds
	// of the speed it was let go at.
	flingCarry = float32(0.25)
	// columnGap is the room between the strip's columns at the reference
	// type size.
	columnGap = 24
	// magnetReach is how near a column's start the strip, come to rest,
	// is pinned to it, at the reference type size; magnetQuiet how long
	// the strip's input is quiet before it counts as come to rest; and
	// magnetPin how long the pin takes, short, so it reads as taken.
	magnetReach = 72
	magnetQuiet = 90 * time.Millisecond
	magnetPin   = 260 * time.Millisecond
	// magnetHeld is how long after the last input fingers still on the
	// touchpad keep the strip from being pinned, should they lift without
	// a word.
	magnetHeld = 400 * time.Millisecond
)

// Mode is how the window lays the notebook out: the page beside its
// panes, or the canvas over them. It is the window's top choice, as the
// panes are what the page mode shows.
type Mode uint8

const (
	ModePage Mode = iota
	ModeCanvas
)

// span is a column's place along the strip, from the strip's start.
type span struct{ x, w float32 }

type canvasState struct {
	// strip is where the strip is scrolled to, and over how far past its
	// ends it is pulled; viewport is its window, spans its columns as last
	// laid out.
	strip    ui.ScrollState
	over     scrollfx.Overscroll
	viewport ui.Rect
	spans    []span
	// inputs counts the strip's input; touching is whether fingers are on
	// the touchpad; still is where the strip stood the build before, and
	// armed whether it was scrolled by hand since it was last pinned
	// (magnet).
	inputs   int
	touching bool
	still    float32
	armed    bool
	// front is the sheet in front as the frame began, and follow whether
	// it came in front since the last, so the strip goes to it; entered is
	// the canvas coming, which brings the sheet in front into view at once.
	front   string
	follow  bool
	entered bool
	// reveal is a sheet to bring into view, as one opened from the map
	// that was open already.
	reveal string
	// scrub is the scrubber's (canvas_scrub.go).
	scrub scrubState
	// columns are the strip's, in order: the open sheets and the pickers
	// among them (arrange); pickers counts the pickers made, for their ids.
	// into and beside are sheets on their way from a picker, by its id: to
	// take its place, or to stand after it. placePicker asks for a picker
	// before the sheet in front, as the canvas comes, and revealPicker is
	// one to bring into view.
	columns      []canvasColumn
	pickers      int
	into, beside map[string]int
	placePicker  bool
	revealPicker int
	// listPicker is the picker the canvas came with, which moves out from
	// the sheet list (morph).
	listPicker int
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
	s.canvas.placePicker = m == ModeCanvas
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

// canvasView draws the canvas over the window, shown as far as shown
// says: it fades in over the page mode's layout as it comes, what it shares
// with that layout moving on top; going, it fades out as a whole.
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, shown float32, animate bool) {
	s.canvas.follow, s.canvas.front = s.canvas.front != s.ActivePage, s.ActivePage
	s.arrange()
	scale := s.Settings.UIFontSize
	head := layout.Titlebar * scale / theme.DefaultUIFontSize
	canvas := ui.Column(c).Key("canvas").Absolute().Top(0).Left(0).Right(0).Bottom(0).ZIndex(1)
	// Going, it fades as a whole, and leaves the pointer to the page.
	if !s.CanvasOpen() {
		canvas.Opacity(shown).IgnoreInput(true)
	}
	canvas.Children(func() {
		// The ground comes in under the columns, which come on their own:
		// absolute, it would paint over them but for its z-index.
		ui.Box(c).Key("canvas-ground").Absolute().Top(0).Left(0).Right(0).Bottom(0).ZIndex(-1).
			Background(semantic.Surface.Background).Opacity(shown).PassThrough()
		s.canvasStrip(c, semantic, layout, head, animate)
		s.scrubber(c, semantic, layout.ContentGutter*scale/theme.DefaultUIFontSize, animate)
		// The room over the strip moves the window, as a title bar would.
		ui.Box(c).Key("canvas-drag").Absolute().Top(0).Left(0).Right(0).Height(head).DragWindow()
	})
	// The canvas has come once it shows whole.
	s.canvas.entered = s.canvas.entered && shown < 0.999
	s.canvas.reveal = ""
}

// canvasStrip draws the strip's columns under head's room.
func (s *State) canvasStrip(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	gutter := layout.ContentGutter * k
	strip := ui.ScrollHorizontal(c).Key("canvas-strip").Grow(1).MinHeight(0).WidthPercent(100).
		Scrollbars(false).TrackScroll(&s.canvas.strip)
	s.canvas.viewport = strip.LayoutBounds()
	// The strip goes where it is moved, and stretches past its ends as the
	// lists do; its input is counted, and the fingers followed, for the
	// magnet.
	strip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputScroll {
			return false
		}
		s.canvas.inputs++
		s.canvas.armed = true
		s.canvas.touching = scrollfx.Phase(ev.Precise, ev.Momentum) == scrollfx.Direct && (ev.DX != 0 || ev.DY != 0)
		return components.OverscrollPanel.Handle(&s.canvas.over, ev, &s.canvas.strip, scrollfx.Horizontal,
			s.canvas.viewport.W, scale)
	})
	s.magnet(strip, k, animate)
	// The columns end short of the writing actions floating under them,
	// and their heads stand clear of the window's controls over them.
	foot := layout.DockClearance() * k
	top := max(head, gutter/2, s.controlsFoot)
	strip.Padding(top, gutter, foot, gutter)
	window, _ := c.Size()
	editor := s.editorWidth(window-2*gutter, k)
	picker := s.paneLayout().SheetList * k
	strip.Children(func() {
		flow := ui.Row(c).Key("canvas-flow").FillHeight().AlignItems(ui.Stretch).Gap(columnGap * k)
		s.canvas.over.Apply(c, flow, animate)
		origin := flow.LayoutBounds().X
		s.pan(flow)
		pages := make(map[string]Page, len(s.Open))
		for _, page := range s.Open {
			pages[page.ID] = page
		}
		flow.Children(func() {
			s.canvas.spans = s.canvas.spans[:0]
			for _, col := range s.canvas.columns {
				if col.picker != nil {
					s.pickerColumn(c, semantic, col.picker, picker, origin, animate)
					continue
				}
				s.openColumn(c, semantic, pages[col.sheet], editor, origin, animate)
			}
			if len(s.canvas.columns) == 0 {
				s.sheetHint(c, semantic, editor, origin, animate)
			}
		})
	})
}

// pan has a drag on el move the strip, and a drag let go carry it on, to
// pin at a column's start it comes near. It reports a drag let go, whose
// release is no click.
func (s *State) pan(el ui.Element) bool {
	if dx, _, ok := el.Dragged(); ok && dx != 0 {
		s.stripAt(s.canvas.strip.X - dx)
	}
	vx, _, ok := el.DragEnded()
	if ok {
		s.settleStrip(s.canvas.strip.X - vx*flingCarry)
	}
	return ok
}

// stripAt puts the strip at x at once, as a drag moves it, taking it from
// any glide on its way.
func (s *State) stripAt(x float32) {
	s.canvas.strip.ScrollTo(max(0, min(x, s.canvas.strip.MaxX)), 0, ui.Animation{Duration: 1})
}

// magnet pins the strip to a column's start once, scrolled by hand, it has
// come to rest by itself near one: its input quiet a moment, the fingers
// off the touchpad, and it no longer moving. Nowhere near one, it stays
// where it stopped. While it moves it is left alone: the pull is felt only
// at the end, all at once, as a magnet takes what has come near it. Where
// the canvas moves the strip itself, it goes where it is sent.
func (s *State) magnet(strip ui.Element, k float32, animate bool) {
	x := s.canvas.strip.X
	// The value reaches the input so far magnetQuiet after the last.
	inputs := float32(s.canvas.inputs)
	quiet := strip.AnimateTo("magnet-quiet", inputs, ui.Animation{Duration: magnetQuiet}) == inputs
	// Fingers that lifted without saying so are taken to have lifted a
	// while after the last input.
	held := s.canvas.touching &&
		strip.AnimateTo("magnet-held", inputs, ui.Animation{Duration: magnetHeld}) != inputs
	moved := x != s.canvas.still
	s.canvas.still = x
	if moved || !s.canvas.armed || !quiet || held || strip.Dragging() {
		return
	}
	s.canvas.armed = false
	if rest, ok := s.nearRest(x, magnetReach*k); ok && math.Abs(float64(rest-x)) > 0.5 {
		s.canvas.strip.ScrollTo(rest, 0, s.pin(animate))
	}
}

// pin is how the strip goes to a column it is pinned to.
func (s *State) pin(animate bool) ui.Animation {
	if !animate {
		return ui.Animation{Duration: 1}
	}
	return components.Spring(magnetPin)
}

// nearRest is the column start nearest x, the strip's own ends among them,
// if it is within reach.
func (s *State) nearRest(x, reach float32) (float32, bool) {
	best, found := float32(0), false
	for _, at := range append(s.rests(), 0, s.canvas.strip.MaxX) {
		if d := math.Abs(float64(at - x)); d <= float64(reach) && (!found || d < math.Abs(float64(best-x))) {
			best, found = at, true
		}
	}
	return best, found
}

// rests are where the strip rests at each column, its start at the
// window's, as far as the strip goes.
func (s *State) rests() []float32 {
	out := make([]float32, 0, len(s.canvas.spans))
	for _, sp := range s.canvas.spans {
		out = append(out, min(sp.x, s.canvas.strip.MaxX))
	}
	return out
}

// editorWidth is a sheet's column's width in a window inner wide: half of
// it less the gap between, or a third where a third still holds a sheet's
// page and the room beside it.
func (s *State) editorWidth(inner, k float32) float32 {
	gap := columnGap * k
	page := s.Settings.Prose().Column() + 2*sheetPad*k
	n := float32(2)
	if (inner-2*gap)/3 >= page {
		n = 3
	}
	return max(0, (inner-(n-1)*gap)/n)
}

// column is one of the strip's columns, keyed key, w wide and as tall as
// the strip; its place is counted for the magnet and the scrubber. origin
// is where the strip's columns start in the window. One that comes with
// the canvas, carrying what the page showed, comes in no way of its own
// (enters false).
func (s *State) column(c *ui.Context, key string, w, origin float32, enters, animate bool) ui.Element {
	col := ui.Column(c).Key(key).Width(w).Shrink(0).FillHeight()
	if enters {
		col.Transition(columnTransition(animate))
	} else {
		col.Transition(ui.ElementTransition{Duration: motionReveal, Position: true})
	}
	if b := col.LayoutBounds(); b.W > 0 {
		s.canvas.spans = append(s.canvas.spans, span{x: b.X - origin, w: b.W})
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

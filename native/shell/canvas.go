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
// window, each as tall as it, that scrolls sideways without end: the
// columns of a workspace kept from one visit to the next
// (canvas_workspace.go), sheets written in where they stand, and the
// pickers among them that sheets are chosen in (canvas_pickers.go).
//
// The canvas and the page mode are two scenes of the whole window,
// crossing as the mode changes (components.SceneLayer): the one gone to
// over the other, as opaque as far as it has come, nothing laid out
// again. The sheet in front, the one thing they share, flies from one to
// the other over both (sheetSurface).
//
// A sheet's column is half the window, less the gap between, so two
// neighbours read as a split, or a third where a third still holds a
// sheet's page whole; a picker's is the sheet list's width. Widths change
// only as the window does, never as the strip moves or the mode changes,
// and go from halves to thirds smoothly.
//
// Everything on the strip moves on one motion (canvasMotion): a column
// made opens out where it stands as its neighbours make room, one closed
// folds away as they close up, a sheet coming in front is travelled to,
// and the strip's own moves, to a column pressed in the overview or one
// it comes to rest near, go the same way.
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
	// columnGap is the room between the strip's columns, and canvasEdge
	// that between them and the window's edges, at the reference type
	// size: little, so the columns take the window.
	columnGap  = 10
	canvasEdge = 10
	// magnetReach is how near a column's start the strip, come to rest,
	// is pinned to it, at the reference type size; magnetQuiet how long
	// the strip's input is quiet before it counts as come to rest.
	magnetReach = 72
	magnetQuiet = 90 * time.Millisecond
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

// String names the mode, as where the sheet in front stands
// (components.Sharing).
func (m Mode) String() string {
	if m == ModeCanvas {
		return "canvas"
	}
	return "page"
}

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
	// front is the sheet in front as the frame began; entered is the
	// canvas coming, which brings the sheet in front into view at once.
	front   string
	entered bool
	// bring is the key of a column to bring into view as it is built, at
	// once with bringAtOnce (bringInView).
	bring       string
	bringAtOnce bool
	// scrub is the scrubber's (canvas_scrub.go).
	scrub scrubState
	// workspaces are the canvas's rows, kept as the mode changes, and
	// shown the one it shows (canvas_workspace.go). pickers counts the
	// pickers made, for their ids. into and beside are sheets on their way
	// from a picker, by its id: to take its place, or to stand after it.
	workspaces   []*canvasWorkspace
	shown        int
	pickers      int
	into, beside map[string]int
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

// SetMode lays the notebook out as m: the canvas, or the page with its
// panes as they were, the sheet in front handed from one to the other
// (canvas_workspace.go).
func (s *State) SetMode(m Mode) {
	if s == nil || s.mode == m {
		return
	}
	if s.Library != nil {
		if m == ModeCanvas {
			s.enterCanvas()
		} else {
			s.leaveCanvas()
		}
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

// canvasView is the canvas's scene, crossing with the page's as shown
// says (components.SceneLayer).
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, shown float32, animate bool) {
	s.keepCanvas()
	scale := s.Settings.UIFontSize
	head := layout.Titlebar * scale / theme.DefaultUIFontSize
	scene := ui.Column(c).Key("canvas").Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Background(semantic.Surface.Background)
	components.SceneLayer(scene, shown, s.laid == ModeCanvas).Children(func() {
		s.canvasStrip(c, semantic, layout, head, animate)
		s.scrubber(c, semantic, layout.ContentGutter*scale/theme.DefaultUIFontSize, animate)
		// The room over the strip moves the window, as a title bar would.
		ui.Box(c).Key("canvas-drag").Absolute().Top(0).Left(0).Right(0).Height(head).DragWindow()
	})
	// The canvas has come once it shows whole.
	s.canvas.entered = s.canvas.entered && shown < 0.999
}

// canvasStrip draws the strip's columns under head's room.
func (s *State) canvasStrip(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	edge := canvasEdge * k
	strip := ui.ScrollHorizontal(c).Key("canvas-strip").Grow(1).MinHeight(0).WidthPercent(100).
		Scrollbars(false).TrackScroll(&s.canvas.strip)
	viewport := strip.LayoutBounds()
	// The window resized, the sheet in front stays in view where it was.
	if viewport.W != s.canvas.viewport.W && s.canvas.viewport.W > 0 {
		if front := s.canvasFront(); front != "" {
			s.bringInView(canvasColumn{sheet: front}.key(), true)
		}
	}
	s.canvas.viewport = viewport
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
	strip.Padding(max(head, s.controlsFoot), 0, layout.DockClearance()*k, 0)
	window, _ := c.Size()
	editor := s.editorWidth(strip, window-2*edge, k, animate)
	picker := s.paneLayout().SheetList * k
	strip.Children(func() {
		// The flow is at least as wide as the window, its room before,
		// between and after the columns its own (room). A column folding
		// away takes no room, so as one closes the flow keeps its room a
		// while, easing back with it, rather than the strip being cut back
		// under it at once (components.Settled).
		natural := float32(0)
		if n := len(s.canvas.spans); n > 0 {
			last := s.canvas.spans[n-1]
			natural = last.x + last.w + 2*edge
		}
		flow := ui.Row(c).Key("canvas-flow").FillHeight().AlignItems(ui.Stretch)
		extent := components.Settled(flow, "extent", natural, components.MotionEnter, animate)
		flow.MinWidth(max(viewport.W, extent))
		s.canvas.over.Apply(c, flow, animate)
		// The columns' places are counted from where the first stands.
		origin := flow.LayoutBounds().X + edge
		s.pan(flow)
		pages := make(map[string]Page, len(s.Open))
		for _, page := range s.Open {
			pages[page.ID] = page
		}
		flow.Children(func() {
			s.canvas.spans = s.canvas.spans[:0]
			columns := s.workspace().columns
			s.room(c, "room-start", 0, edge, false)
			if len(columns) == 0 {
				s.sheetHint(c, semantic, editor, animate)
				s.room(c, "room-end", 0, edge, true)
			}
			for i, col := range columns {
				if col.picker != nil {
					s.pickerColumn(c, semantic, col.picker, picker, origin, animate)
				} else {
					s.openColumn(c, semantic, pages[col.sheet], editor, origin, animate)
				}
				// The room after the last reaches the window's end.
				last := i == len(columns)-1
				w := columnGap * k
				if last {
					w = edge
				}
				s.room(c, "room-"+col.key(), i+1, w, last)
			}
		})
	})
}

// room is empty room on the canvas, w wide, or more where it may grow,
// before its workspace's column at: unseen, and, right-clicked, it offers
// to make a picker there.
func (s *State) room(c *ui.Context, key string, at int, w float32, grow bool) {
	room := ui.Box(c).Key(key).Width(w).Shrink(0).FillHeight().Label("Canvas")
	if grow {
		room.Grow(1)
	}
	room.ContextMenu(func(m *ui.Menu) {
		if m.Item("New Picker").Chosen() {
			s.newPickerAt(at)
		}
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
		s.canvas.strip.ScrollTo(rest, 0, canvasMotion(animate))
	}
}

// canvasMotion is how the strip moves of its own accord: travelling to a
// column, or to rest, as far as it goes, in the time its columns take to
// open out or fold away (columnTransition), so the two arrive together.
func canvasMotion(animate bool) ui.Animation {
	if !animate {
		return ui.Animation{Duration: 1}
	}
	return components.Spring(components.MotionEnter)
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
// page and the room beside it. Between the two it goes smoothly, as the
// window is resized across where they meet, on el.
func (s *State) editorWidth(el ui.Element, inner, k float32, animate bool) float32 {
	gap := columnGap * k
	page := s.Settings.Prose().Column() + 2*sheetPad*k
	n := float32(2)
	if (inner-2*gap)/3 >= page {
		n = 3
	}
	if animate {
		n = el.AnimateTo("columns-per-window", n, canvasMotion(animate))
	}
	return max(0, (inner-(n-1)*gap)/n)
}

// column is one of the strip's columns, w wide and as tall as the strip,
// and whether it is in front, its sheet the sheet in front; its place is counted for
// the magnet and the scrubber, and it is brought into view where asked
// (bringInView). origin is where the strip's columns start in the window.
func (s *State) column(c *ui.Context, of canvasColumn, w, origin float32, animate bool) (ui.Element, bool) {
	key := of.key()
	col := ui.Column(c).Key(key).Width(w).Shrink(0).FillHeight().Transition(columnTransition(animate))
	if b := col.LayoutBounds(); b.W > 0 {
		s.canvas.spans = append(s.canvas.spans, span{x: b.X - origin, w: b.W})
	}
	if s.canvas.bring == key {
		motion := canvasMotion(animate && !s.canvas.bringAtOnce)
		components.Reveal(c, col, canvasEdge*components.UIFontSize(c)/theme.DefaultUIFontSize,
			ui.ScrollIntoViewOptions{Inline: ui.ScrollNearest, Animation: &motion})
		s.canvas.bring = ""
	}
	return col, of.picker == nil && of.sheet == s.ActivePage
}

// columnTransition is how the strip's columns come and go: opening out
// where they stand, from nothing across, as they fade and rise to their
// size, and folding away the same way, those beside them making room and
// closing up on the same motion, a little past and back. What a column
// holds keeps its layout as it opens or folds, clipped, so nothing in it
// is set anew on the way.
func columnTransition(animate bool) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	spring := components.Bouncy(components.MotionEnter)
	gone := &ui.Motion{Collapse: true, Opacity: 0, Scale: 0.96}
	return ui.ElementTransition{Animation: &spring, Position: true, Colors: true, Enter: gone, Exit: gone}
}

// columnRadius is how round a column's corners are, at the reference
// type size.
const columnRadius = 10

// columnEdge is a column's edge: the accent's, softly, for the one in
// front, and a faint line for the rest.
func columnEdge(semantic theme.Theme, front bool) ui.Color {
	if front {
		return semantic.Accent.Foreground.Alpha(0.55)
	}
	return semantic.Border.Variant.Alpha(0.6)
}

// bringInView has the strip bring the column keyed key into view, the gap
// beside it kept from the window's edge (components.Reveal), as it is
// next built, which may be once it joins: travelling there, or at once.
func (s *State) bringInView(key string, atOnce bool) {
	if key != "" {
		s.canvas.bring, s.canvas.bringAtOnce = key, atOnce
	}
}

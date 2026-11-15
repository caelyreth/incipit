package shell

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The canvas is the notebook laid out as one flow, where the columns show
// one place and one sheet at a time: a strip of columns across the window,
// each as tall as it, that scrolls sideways without end. The first column
// is the map, every group a region of its sheets' cards; after it stands a
// column for each open sheet, written in where it stands, as many as are
// open. A touchpad's swipe, Shift and the wheel, or a drag on the strip
// moves along it, and it comes to rest with a column's start at its own;
// the sheet in front is always brought into view.
//
// The strip zooms as one: nearer, every column is wider and its type
// larger; farther, more columns show. The columns keep the window's
// height. A zoom glides as a scale of what is drawn, and lays the strip
// out at its end, at a size of a few (zoomGrain), as type is not to be
// animated through its sizes.
//
// The map is canvas_map.go, the sheets' columns canvas_sheets.go.

// The canvas's zoom keeps between these, 1 setting it at the window's own
// size.
const (
	minZoom = float32(0.3)
	maxZoom = float32(1.6)
	// zoomStep is how far the head's buttons go nearer or farther.
	zoomStep = float32(1.25)
	// zoomGrain is what the zoom the strip is laid out at is a multiple of,
	// so that its type is set at few sizes.
	zoomGrain = float32(0.05)
	// pinchRest is how long a pinch rests before the strip is laid out at
	// the zoom it reached.
	pinchRest = 150 * time.Millisecond
	// flingCarry is how far a drag let go carries the strip on, in seconds
	// of the speed it was let go at.
	flingCarry = float32(0.25)
	// columnGap is the room between the strip's columns at the reference
	// type size.
	columnGap = 24
)

type canvasState struct {
	open bool
	// zoom is how near the strip is laid out, 0 for 1; pinch, while
	// pinching, is how near it is shown, about anchor, a point of the
	// strip's window, and pinches counts the pinch's moves, the last of
	// which, once it rests, lays the strip out there.
	zoom     float32
	pinching bool
	pinch    float32
	anchor   float32
	pinches  int
	// relaid is set while a frame lays the strip out at a new zoom: what
	// moves there is the zoom's glide, not the columns' own motion.
	relaid bool
	// query lights the sheets it finds, and dims the rest, in place;
	// input is its field's, focus asks the field for the focus, and typing
	// is whether it has it.
	query  string
	input  queryInput
	focus  bool
	typing bool
	// strip is where the strip is scrolled to; viewport is its window,
	// inner the width it shows its columns in, width all it holds, and
	// rests where it rests at each column, as last laid out.
	strip    ui.ScrollState
	viewport ui.Rect
	inner    float32
	width    float32
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

// CanvasOpen reports whether the canvas shows in place of the columns.
func (s *State) CanvasOpen() bool { return s != nil && s.canvas.open && s.Library != nil }

// ToggleCanvas goes to the canvas, zoomed as it was left, the sheet in
// front where the page stood, or back to the columns as they were.
func (s *State) ToggleCanvas() {
	if s == nil {
		return
	}
	if s.canvas.open {
		s.closeCanvas()
		return
	}
	s.canvas.open, s.canvas.typing, s.canvas.entered = true, false, true
}

// closeCanvas goes back to the columns, as choosing panes does.
func (s *State) closeCanvas() {
	s.canvas.open, s.canvas.typing, s.canvas.pinching = false, false, false
}

// scale is how near the strip is laid out.
func (c canvasState) scale() float32 {
	if c.zoom == 0 {
		return 1
	}
	return c.zoom
}

// zoomTo lays the strip out at next, to a multiple of zoomGrain within the
// zoom's range, keeping at, a point of its window, where it stands. What
// is drawn glides there from where it was shown.
func (s *State) zoomTo(next, at float32) {
	next = float32(math.Round(float64(next/zoomGrain))) * zoomGrain
	next = max(minZoom, min(next, maxZoom))
	z := s.canvas.scale()
	if next == z {
		return
	}
	// Everything across the strip scales with the zoom, its padding too.
	s.canvas.strip.X = max(0, (s.canvas.strip.X+at)*next/z-at)
	s.canvas.zoom, s.canvas.anchor, s.canvas.relaid = next, at, true
}

// zoomCentered goes k times nearer about the middle of the strip's window.
func (s *State) zoomCentered(k float32) {
	s.zoomTo(s.canvas.scale()*k, s.canvas.viewport.W/2)
}

// fit zooms so the whole strip shows, and goes to its start.
func (s *State) fit() {
	if s.canvas.viewport.W <= 0 || s.canvas.width <= 0 {
		return
	}
	s.zoomTo(s.canvas.scale()*s.canvas.viewport.W/s.canvas.width, 0)
	s.canvas.strip.ScrollTo(0, 0, components.Spring(components.MotionScene))
}

// pinched takes a pinch's move: the strip shows scale times as near about
// x, and is laid out there once the pinch rests.
func (s *State) pinched(scale, x float32) {
	if !s.canvas.pinching {
		s.canvas.pinching, s.canvas.pinch = true, s.canvas.scale()
	}
	s.canvas.pinch = max(minZoom, min(s.canvas.pinch*scale, maxZoom))
	s.canvas.anchor = x
	s.canvas.pinches++
}

// canvasView draws the canvas: its head, over the strip, with the writing
// actions floating over its foot for the sheet in front. head is the room
// over it kept for the window's traffic lights, as the panes before it
// close.
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	s.canvas.follow, s.canvas.front = s.canvas.front != s.ActivePage, s.ActivePage
	lit, found := s.canvasLit()
	gutter := layout.ContentGutter * scale / theme.DefaultUIFontSize
	canvas := ui.Column(c).Key("canvas").Grow(1).MinWidth(0).Background(semantic.Surface.Background).
		Transition(fadeTransition(animate))
	canvas.Children(func() {
		s.canvasHead(c, semantic, head, gutter, lit != nil, found, animate)
		s.canvasStrip(c, semantic, layout, lit, animate)
		if s.ActivePage != "" {
			s.dock(c, semantic, theme.DefaultControls.Scaled(scale))
		}
	})
	// The sheet in front keeps the middle while the panes close.
	s.canvas.entered = s.canvas.entered && s.lastSidebarWidth > 0.5
	s.canvas.relaid, s.canvas.reveal = false, ""
}

// canvasStrip draws the strip: the map, then the open sheets' columns,
// shown at the zoom's glide.
func (s *State) canvasStrip(c *ui.Context, semantic theme.Theme, layout theme.Layout, lit map[string]bool, animate bool) {
	scale := s.Settings.UIFontSize
	gutter := layout.ContentGutter * scale / theme.DefaultUIFontSize
	strip := ui.ScrollHorizontal(c).Key("canvas-strip").Grow(1).MinHeight(0).WidthPercent(100).
		ScrollSnap(ui.SnapProximity).TrackScroll(&s.canvas.strip)
	s.canvas.viewport = strip.LayoutBounds()
	// A pinch that rested lays the strip out where it went, so the zoom is
	// read after.
	shown := s.shownZoom(strip, animate)
	z := s.canvas.scale()
	// The columns end short of the writing actions floating under them.
	foot := gutter
	if s.ActivePage != "" {
		foot = layout.DockClearance() * scale / theme.DefaultUIFontSize
	}
	pad := gutter * z
	s.canvas.inner = s.canvas.viewport.W - 2*pad
	strip.Padding(gutter/2, pad, foot, pad)
	// A pinch, or a wheel turned with Ctrl, zooms about the pointer.
	strip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputZoom {
			return false
		}
		s.pinched(ev.Scale, ev.X)
		return true
	})
	// What moves as the strip is laid out at a new zoom is the zoom's
	// glide, not the columns' own motion; the columns there as the canvas
	// opens come with it, not one by one.
	columns := animate && !s.canvas.relaid && !s.canvas.entered
	strip.Children(func() {
		// Inside, the type is the zoom's: every size on the strip follows
		// components.UIFontSize.
		zoomed := *s.Settings.MyGoThemeFor(semantic)
		zoomed.FontSize = scale * z
		ui.Themed(c, &zoomed).Key("canvas-zoom").FillHeight().Children(func() {
			k := components.UIFontSize(c) / theme.DefaultUIFontSize
			flow := ui.Row(c).Key("canvas-flow").FillHeight().AlignItems(ui.Stretch).Gap(columnGap * k)
			bounds := flow.LayoutBounds()
			s.canvas.width = bounds.W + 2*pad
			// What is drawn glides to the zoom laid out, about the anchor
			// across and the columns' heads down: the columns keep the
			// window's height at every zoom, so their tops stay put.
			if r := shown / z; r != 1 && bounds.W > 0 {
				flow.TransformOrigin((s.canvas.strip.X+s.canvas.anchor-pad)/bounds.W, 0).Scale(r, r)
			}
			s.pan(flow)
			flow.Children(func() {
				s.canvas.rests = s.canvas.rests[:0]
				s.canvasMap(c, semantic, lit, pad, bounds.X, columns)
				for _, page := range s.Open {
					s.openColumn(c, semantic, page, bounds.X, columns)
				}
				if len(s.Open) == 0 {
					s.sheetHint(c, semantic, bounds.X, columns)
				}
			})
		})
	})
}

// shownZoom is how near the strip is shown: where a pinch has it, or
// gliding to where it is laid out. A pinch that rested lays it out there.
func (s *State) shownZoom(strip ui.Element, animate bool) float32 {
	z := s.canvas.scale()
	if !s.canvas.pinching {
		// The rest counts from the pinches so far.
		strip.AnimateTo("pinch-rest", float32(s.canvas.pinches), ui.Animation{Duration: pinchRest})
		if !animate {
			return strip.Jump("zoom", z)
		}
		return components.Follow(strip, "zoom", z, components.MotionPanel)
	}
	rest := strip.AnimateTo("pinch-rest", float32(s.canvas.pinches), ui.Animation{Duration: pinchRest})
	shown := strip.Jump("zoom", s.canvas.pinch)
	if !animate || rest == float32(s.canvas.pinches) {
		s.canvas.pinching = false
		s.zoomTo(s.canvas.pinch, s.canvas.anchor)
	}
	return shown
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

// canvasLit is what the canvas's query finds, by sheet, and how many; nil
// while there is no query.
func (s *State) canvasLit() (map[string]bool, int) {
	if strings.TrimSpace(s.canvas.query) == "" {
		return nil, 0
	}
	result := s.Library.Search(s.canvas.query)
	lit := make(map[string]bool, len(result.Sheets))
	for _, sheet := range result.Sheets {
		lit[sheet.ID] = true
	}
	return lit, result.Total
}

// litAmount is how lit something on the canvas is, gliding as a search
// lights and dims: 1 lit, dim when not.
func litAmount(el ui.Element, lit bool, dim float32, animate bool) float32 {
	target := float32(1)
	if !lit {
		target = dim
	}
	if !animate {
		return target
	}
	return components.Follow(el, "lit", target, components.MotionNormal)
}

// canvasHead is the canvas's line at its top, under head's room for the
// window's traffic lights: the query that lights sheets, what it found,
// and how near the canvas is. It ends short of the window's controls.
func (s *State) canvasHead(c *ui.Context, semantic theme.Theme, head, gutter float32, searching bool, found int, animate bool) {
	scale := s.Settings.UIFontSize
	if head > 0.5 {
		ui.Box(c).Key("canvas-drag").WidthPercent(100).Height(head).Shrink(0).DragWindow()
	}
	line := ui.Row(c).Key("canvas-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space12.DIPs(scale)).PaddingStart(gutter).PaddingEnd(max(gutter, s.controlsWidth)).
		PaddingY(theme.Space12.DIPs(scale))
	if head <= 0.5 {
		line.DragWindow()
	}
	line.Children(func() {
		field, input := queryField{
			Key: "canvas-search", Label: "Light sheets", Placeholder: "Light the sheets a search finds",
			Value: &s.canvas.query, In: &s.canvas.input, Scope: query.InNotebook,
			Focus: s.canvas.focus, Animate: animate,
			Keys: func(k ui.TextKey) bool {
				if k.Key == ui.KeyEscape && k.Mods == 0 && s.canvas.query != "" {
					s.canvas.query = ""
					return true
				}
				return false
			},
		}.View(s, c, semantic)
		field.Grow(1).MaxWidth(440 * scale / theme.DefaultUIFontSize)
		s.canvas.focus = false
		s.canvas.typing = input.Focused()
		if searching {
			components.Label(c, foundCount(found), theme.TextXSmall, semantic.Text.Muted).
				SingleLine().Shrink(0).FontFeatures("tnum").Transition(popTransition(animate))
		}
		ui.Box(c).Grow(1)
		s.zoomControls(c, semantic, animate)
	})
}

// zoomControls go nearer and farther a step at a time, say how near the
// canvas is (which, pressed, sets it at the window's own size), and fit
// the whole of it in view.
func (s *State) zoomControls(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	icon := func(key, label string, glyph components.IconName, action func()) {
		components.IconButton{
			Key: key, Label: label, Tooltip: label, Icon: glyph.SVG(),
			Style: components.SubtleButton, Size: components.CompactButton,
			IconSize: 14 * scale / theme.DefaultUIFontSize, IconColor: &semantic.Icon.Muted,
			Animate: animate, Action: action,
		}.View(c, semantic)
	}
	ui.Row(c).Key("zoom").Shrink(0).AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale)).Children(func() {
		icon("zoom-out", "Zoom out", components.IconMinus, func() { s.zoomCentered(1 / zoomStep) })
		components.Button{
			Key: "zoom-actual", Label: fmt.Sprintf("%d%%", int(s.canvas.scale()*100+0.5)), Tooltip: "Actual size",
			Style: components.SubtleButton, Size: components.CompactButton, Animate: animate,
			Action: func() { s.zoomCentered(1 / s.canvas.scale()) },
		}.View(c, semantic)
		icon("zoom-in", "Zoom in", components.IconPlus, func() { s.zoomCentered(zoomStep) })
		components.Button{
			Key: "zoom-fit", Label: "Fit", Tooltip: "Show the whole canvas",
			Style: components.SubtleButton, Size: components.CompactButton, Animate: animate,
			Action: s.fit,
		}.View(c, semantic)
	})
}

// foundCount says how many sheets a search lit.
func foundCount(n int) string {
	switch n {
	case 0:
		return "Nothing found"
	case 1:
		return "1 sheet found"
	}
	return thousands(n) + " sheets found"
}

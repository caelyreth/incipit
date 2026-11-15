package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The canvas is the notebook laid out on one surface, where the columns
// show one place and one sheet at a time. Its map holds every group as a
// region of cards; its desk, beside the map, holds the sheets open, each
// written in where it lies, as many as are open. The surface pans and
// zooms as one: nearer, everything on it is larger, the pages' type too;
// farther, more of it shows. Nothing on it is placed by hand: the notebook
// arranges it.
//
// The map is canvas_map.go, the desk canvas_desk.go.

// The canvas's zoom keeps between these, 1 setting it at the window's own
// size.
const (
	minZoom = float32(0.3)
	maxZoom = float32(1.6)
	// zoomStep is how far the head's buttons go nearer or farther.
	zoomStep = float32(1.25)
)

type canvasState struct {
	open bool
	// zoom is how near the canvas is, 0 for 1.
	zoom float32
	// query lights the sheets it finds, and dims the rest, in place;
	// input is its field's, focus asks the field for the focus, and typing
	// is whether it has it.
	query  string
	input  queryInput
	focus  bool
	typing bool
	// scroll is where the surface is panned to; viewport and field are the
	// surface's window and all it holds, as last laid out.
	scroll   ui.ScrollState
	viewport ui.Rect
	field    ui.Rect
	// reveal is a sheet just opened on the desk, to bring into view once
	// it is laid out.
	reveal string
}

// CanvasOpen reports whether the canvas shows in place of the columns.
func (s *State) CanvasOpen() bool { return s != nil && s.canvas.open && s.Library != nil }

// ToggleCanvas goes to the canvas, panned and zoomed as it was left, or
// back to the columns as they were.
func (s *State) ToggleCanvas() {
	if s == nil {
		return
	}
	s.canvas.open = !s.canvas.open
	s.canvas.typing = false
}

// closeCanvas goes back to the columns, as choosing panes does.
func (s *State) closeCanvas() {
	s.canvas.open, s.canvas.typing = false, false
}

// scale is how near the canvas is.
func (c canvasState) scale() float32 {
	if c.zoom == 0 {
		return 1
	}
	return c.zoom
}

// zoomAt goes k times nearer, keeping the point at of the field, in its
// own DIPs as laid out now, where it stands in the window.
func (s *State) zoomAt(k float32, at ui.Point) {
	z := s.canvas.scale()
	next := max(minZoom, min(z*k, maxZoom))
	if next == z {
		return
	}
	r := next / z
	s.canvas.scroll.X += at.X * (r - 1)
	s.canvas.scroll.Y += at.Y * (r - 1)
	s.canvas.zoom = next
}

// zoomCentered goes k times nearer about the middle of the surface's
// window.
func (s *State) zoomCentered(k float32) {
	v := s.canvas.viewport
	s.zoomAt(k, ui.Point{X: s.canvas.scroll.X + v.W/2, Y: s.canvas.scroll.Y + v.H/2})
}

// fit zooms so the whole field shows, and goes to its start.
func (s *State) fit() {
	v, f := s.canvas.viewport, s.canvas.field
	if v.W <= 0 || f.W <= 0 || f.H <= 0 {
		return
	}
	z := s.canvas.scale()
	s.canvas.zoom = max(minZoom, min(z*min(v.W/f.W, v.H/f.H), maxZoom))
	s.canvas.scroll.X, s.canvas.scroll.Y = 0, 0
}

// canvasView draws the canvas: its head, over the surface that holds the
// map and the desk, with the writing actions floating over its foot for
// the sheet in front. head is the room over it kept for the window's
// traffic lights, as the panes before it close.
func (s *State) canvasView(c *ui.Context, semantic theme.Theme, layout theme.Layout, head float32, animate bool) {
	scale := s.Settings.UIFontSize
	z := s.canvas.scale()
	lit, found := s.canvasLit()
	gutter := layout.ContentGutter * scale / theme.DefaultUIFontSize
	canvas := ui.Column(c).Key("canvas").Grow(1).MinWidth(0).Background(semantic.Surface.Background).
		Transition(fadeTransition(animate))
	canvas.Children(func() {
		s.canvasHead(c, semantic, head, gutter, lit != nil, found, animate)
		surface := ui.ScrollBoth(c).Key("canvas-surface").Grow(1).MinHeight(0).WidthPercent(100).
			TrackScroll(&s.canvas.scroll)
		s.canvas.viewport = surface.LayoutBounds()
		surface.Children(func() {
			// Inside, the type is the zoom's: every size on the canvas
			// follows components.UIFontSize.
			zoomed := *s.Settings.MyGoThemeFor(semantic)
			zoomed.FontSize = scale * z
			field := ui.Themed(c, &zoomed).Key("canvas-field").Padding(gutter * z)
			s.canvas.field = field.LayoutBounds()
			// A pinch, or a wheel turned with Ctrl, zooms about the pointer.
			field.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputZoom {
					return false
				}
				s.zoomAt(ev.Scale, ui.Point{X: ev.X, Y: ev.Y})
				return true
			})
			// The desk's sheets stand as tall as the window at its own
			// size, short of the writing actions floating under them.
			room := gutter
			if s.ActivePage != "" {
				room = layout.DockClearance() * scale / theme.DefaultUIFontSize
			}
			height := (s.canvas.viewport.H - gutter - room) * z
			field.Children(func() {
				k := components.UIFontSize(c) / theme.DefaultUIFontSize
				ui.Row(c).AlignItems(ui.Start).Gap(64 * k).Children(func() {
					s.canvasMap(c, semantic, lit, animate)
					s.canvasDesk(c, semantic, height, animate)
				})
			})
		})
		if s.ActivePage != "" {
			s.dock(c, semantic, theme.DefaultControls.Scaled(scale))
		}
	})
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
				SingleLine().Shrink(0).FontFeatures("tnum")
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

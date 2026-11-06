package documentview

import (
	"fmt"
	"math"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// A click on a figure selects it as a whole, its line, as the page
// selects anything; typing then replaces it and Delete takes it away,
// while Return goes on below it and Escape leaves it for its caption.
// Its sides resize it: a centered figure grows from both, one at the start
// from its end. Dragging, the page reflows around it as it grows, it snaps
// to the quarters of the column and to its own size, and its width shows
// above it; letting go keeps it in one step of history, and Escape puts
// it back. A screen reader reads it out as it becomes the selection.

const (
	// figureGrip is how near a figure's side, in DIPs, a press resizes it.
	figureGrip = 10
	// figureSnap is how near a size, in DIPs, a resize snaps to it.
	figureSnap = 8
	// figureLeast is the narrowest a resize sets a figure, in DIPs.
	figureLeast = 48
)

// figureResize follows one resize of a figure.
type figureResize struct {
	node model.NodeID
	// side is the side held: -1 the start's, 1 the end's.
	side     float32
	centered bool
	origin   ui.Rect
	column   float32
	// natural is the figure's own width, and span what the resize sets.
	natural float32
	span    float32
}

// figureHover is the figure under the pointer, and the side it is near.
type figureHover struct {
	node model.NodeID
	side float32
	ok   bool
}

// figureAt is the figure at a point of the element, and the side of it
// the point grips, zero for none.
func (e *Element) figureAt(x, y float32) (textlayout.PlacedFigure, float32, bool) {
	if e.current == nil {
		return textlayout.PlacedFigure{}, 0, false
	}
	point := e.contentPoint(x, y)
	figure, ok := e.current.FigureAt(point)
	if !ok {
		return figure, 0, false
	}
	box := figure.Box
	side := float32(0)
	switch {
	case point.X >= box.X+box.W-figureGrip:
		side = 1
	case point.X <= box.X+figureGrip && figure.Align == surface.FigureCenter:
		side = -1
	}
	return figure, side, true
}

// pressFigure handles a press on a figure: on a side it starts a resize,
// elsewhere it selects the figure.
func (e *Element) pressFigure(event ui.InputEvent) (bool, bool) {
	figure, side, ok := e.figureAt(event.X, event.Y)
	e.menuFigure = 0
	if !ok || event.Mods&ui.Shift != 0 {
		return false, false
	}
	if event.Button != 0 {
		// A secondary click selects the figure its menu acts on.
		e.menuFigure = figure.Node
		side = 0
	}
	if side != 0 {
		e.resizing = &figureResize{
			node:     figure.Node,
			side:     side,
			centered: figure.Align == surface.FigureCenter,
			origin:   figure.Box,
			column:   e.current.Width(),
			natural:  float32(figure.Width),
			span:     figure.Box.W / e.current.Width(),
		}
		return true, true
	}
	e.pointer = pointerPressed
	e.pointerAnchor, e.pointerHead = int(figure.Line.Start), int(figure.Line.End)
	if err := e.Input.Select(e.pointerAnchor, e.pointerHead, event.Time); err != nil {
		return true, false
	}
	if bridge, ok := e.Input.(*Bridge); ok && event.Clicks == 2 && event.Button == 0 {
		bridge.viewSelectedFigure()
	}
	return true, true
}

// dragResize follows the pointer with the figure's side.
func (e *Element) dragResize(x float32) {
	resize := e.resizing
	box := resize.origin
	width := x - box.X
	if resize.centered {
		// A centered figure grows from both sides alike.
		width = 2 * float32(math.Abs(float64(x-(box.X+box.W/2))))
	}
	width = max(figureLeast, min(resize.column, width))
	sizes := []float32{resize.column / 4, resize.column / 2, resize.column * 3 / 4, resize.column}
	if resize.natural <= resize.column {
		sizes = append(sizes, resize.natural)
	}
	for _, size := range sizes {
		if float32(math.Abs(float64(width-size))) <= figureSnap {
			width = size
			break
		}
	}
	resize.span = width / resize.column
}

// finishResize keeps the size a resize set, in one step of history: its
// own size as none, so it follows the column no more.
func (e *Element) finishResize() {
	resize := e.resizing
	e.resizing = nil
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return
	}
	span := resize.span
	if resize.natural <= resize.column && math.Abs(float64(span*resize.column-resize.natural)) < 0.5 {
		span = 0
	}
	_, _ = bridge.Controller.ShapeImage(resize.node, func(shape *editor.ImageShape) { shape.Span = span })
}

// resizeLabel is what the width of a figure being resized reads.
func (r *figureResize) label() string {
	if r == nil {
		return ""
	}
	if r.natural <= r.column && math.Abs(float64(r.span*r.column-r.natural)) < 0.5 {
		return "Original size"
	}
	return fmt.Sprintf("%d%%", int(math.Round(float64(r.span*100))))
}

// resizedSpans are the shares of the column a resize under way sets its
// figure at, so the page reflows around it before the size is kept.
func (e *Element) resizedSpans() map[model.NodeID]float32 {
	if e.resizing == nil {
		return nil
	}
	return map[model.NodeID]float32{e.resizing.node: e.resizing.span}
}

// selectedFigure is the figure the selection is, if it is one's whole
// line.
func (e *Element) selectedFigure() (textlayout.PlacedFigure, bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil || e.current == nil {
		return textlayout.PlacedFigure{}, false
	}
	selection := bridge.Controller.Selection().Range
	for _, figure := range e.current.Figures() {
		if figureSelected(figure, model.NewUTF16Range(model.UTF16Offset(selection.Start), model.UTF16Offset(selection.End))) {
			return figure, true
		}
	}
	return textlayout.PlacedFigure{}, false
}

// figureGesture takes the keys a figure's own gestures hold before the
// keymap: a resize takes every key, Escape setting it down, and Space on a
// selected figure looks at its picture, as the system's file views do.
// Leaving it, or breaking below it, is the engine's (SelectionObject).
func (e *Element) figureGesture(event ui.InputEvent) bool {
	if e.resizing != nil {
		if event.Key == ui.KeyEscape {
			e.resizing = nil
		}
		return true
	}
	if event.Key != ui.KeySpace || event.Mods != 0 {
		return false
	}
	if _, ok := e.selectedFigure(); !ok {
		return false
	}
	return e.Input.(*Bridge).viewSelectedFigure()
}

// figureAnnouncement is what a screen reader reads as a figure becomes
// the selection: that it is a picture, and what it shows.
func figureAnnouncement(figure textlayout.PlacedFigure) string {
	if figure.Alt != "" {
		return "Image, " + figure.Alt
	}
	return "Image"
}

package textlayout

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// A figure stands above its caption's line, set apart from the text
// around it as a run of entries is, a little more, and held close to its
// caption, which reads as its own. It takes its room from its size in
// pixels, so the page never moves as its pixels arrive.
const (
	// figureSpace sets a figure and its caption apart from the blocks
	// around them.
	figureSpace = 0.75
	// figureCaption is the room between a figure and its caption.
	figureCaption = 0.35
	// figureTallest bounds a figure's height, as a share of the column's
	// width: a long screenshot shows narrower rather than pages tall.
	figureTallest = 1.25
	// figureNarrowest is the least a figure is set at, in DIPs, so a tiny
	// picture still shows as one.
	figureNarrowest = 24
	// figureObjectWidth is the width of the character a figure's line
	// opens with, in DIPs: an object as narrow as one is and still is.
	figureObjectWidth = 1e-3
)

// captionRoom is the share of its line a figure's caption takes, closed
// to none while it has no caption.
func captionRoom(figure surface.Figure) float32 {
	return max(0, min(1, figure.Caption))
}

// figureObject sets the character a figure's line opens with, its
// picture's, as an inline object that draws nothing and takes no room, so
// the caption stands as if alone on its line.
func figureObject(spans []ui.Span) []ui.Span {
	if len(spans) == 0 || !strings.HasPrefix(spans[0].Text, surface.FigureObject) {
		return spans
	}
	object, rest := spans[0], spans[0]
	object.Text = surface.FigureObject
	object.ObjectWidth = figureObjectWidth
	rest.Text = rest.Text[len(surface.FigureObject):]
	objects := []ui.Span{object}
	if rest.Text != "" {
		objects = append(objects, rest)
	}
	return append(objects, spans[1:]...)
}

func figureGap(body float32) float32 {
	return float32(math.Round(float64(body * figureSpace)))
}

func figureBond(body float32) float32 {
	return float32(math.Round(float64(body * figureCaption)))
}

// figureBox is where a figure stands in a column of width, its top at
// zero: its span of the column, or its own size, as wide as the column at
// most; no taller than figureTallest of it, keeping its proportions.
func figureBox(figure surface.Figure, column float32) ui.Rect {
	if column <= 0 || figure.Width <= 0 || figure.Height <= 0 {
		return ui.Rect{}
	}
	ratio := float32(figure.Height) / float32(figure.Width)
	width := float32(figure.Width)
	if figure.Span > 0 {
		width = figure.Span * column
	}
	width = min(column, max(width, min(column, figureNarrowest)))
	height := width * ratio
	if tallest := column * figureTallest; height > tallest {
		height, width = tallest, tallest/ratio
	}
	width, height = float32(math.Round(float64(width))), float32(math.Round(float64(height)))
	x := float32(0)
	if figure.Align == surface.FigureCenter {
		x = float32(math.Round(float64((column - width) / 2)))
	}
	return ui.Rect{X: x, W: width, H: height}
}

// PlacedFigure is a figure as the layout sets it: its box, content-local,
// and the line it stands above, from its start to its end before the
// break.
type PlacedFigure struct {
	surface.Figure
	Box  ui.Rect
	Line model.UTF16Range
}

// Figures are the figures the layout sets, in order.
func (l *Layout) Figures() []PlacedFigure {
	if l == nil {
		return nil
	}
	var placed []PlacedFigure
	for index := range l.paragraphs {
		if figure, ok := l.paragraphs[index].placed(); ok {
			placed = append(placed, figure)
		}
	}
	return placed
}

// FigureAt is the figure whose box holds point, content-local.
func (l *Layout) FigureAt(point ui.Point) (PlacedFigure, bool) {
	paragraph := l.paragraphAtY(point.Y)
	if paragraph == nil {
		return PlacedFigure{}, false
	}
	figure, ok := paragraph.placed()
	box := figure.Box
	if !ok || point.X < box.X || point.X > box.X+box.W || point.Y < box.Y || point.Y > box.Y+box.H {
		return PlacedFigure{}, false
	}
	return figure, true
}

func (p *Paragraph) placed() (PlacedFigure, bool) {
	if !p.hasFigure {
		return PlacedFigure{}, false
	}
	return PlacedFigure{
		Figure: p.figure,
		Box:    p.box,
		Line:   model.NewUTF16Range(model.UTF16Offset(p.startUTF16), model.UTF16Offset(p.contentUTF16)),
	}, true
}

// BlockEdge is the height of the edge below the block holding offset,
// midway into the room before the next, content-local: where what goes
// after the block would go.
func (l *Layout) BlockEdge(offset model.UTF16Offset) float32 {
	paragraph, _ := l.paragraphAt(offset)
	if paragraph == nil {
		return l.height
	}
	bottom := paragraph.top + paragraph.height
	for index := range l.paragraphs {
		if &l.paragraphs[index] == paragraph && index+1 < len(l.paragraphs) {
			next := l.paragraphs[index+1]
			// The room before the next block's figure, not the figure.
			room := next.gap
			if next.hasFigure {
				room = next.box.Y - bottom
			}
			return bottom + room/2
		}
	}
	return bottom
}

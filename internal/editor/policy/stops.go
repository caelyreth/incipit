package policy

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Stops is a view's stop model: the caret stops the presentation defines,
// in logical order. The engine owns it and builds it from the presentation,
// so logical motion in policies lands exactly where the renderer shows
// carets. Motion reaches every stop, an object's own slots' among them.
type Stops interface {
	// Step is the stop next to c in dir, or false at the page's edge.
	Step(c doc.Caret, dir vocab.Direction) (doc.Caret, bool)
	// Edge is the page's first stop (Backward) or last stop (Forward).
	Edge(dir vocab.Direction) (doc.Caret, bool)
	// Normalize is the valid stop nearest c: c itself when it is a stop.
	Normalize(c doc.Caret) doc.Caret
	// Around is a position's first and last stop: what is projected
	// there, below and above its own stop 0.
	Around(p doc.Pos) (first, last int)
	// Bound is the document position c stands for as the end of a range:
	// its own position, except that the stop before a block's marker
	// stands before the block, at the gap in the slot holding it.
	Bound(c doc.Caret) doc.Pos
}

// StopsFunc builds the stop model of a document as a view sees it.
type StopsFunc func(r doc.Reader, view *View) Stops

// Stops is the stop model of the context's document and view.
func (c *Ctx) Stops() Stops {
	if c.stops == nil {
		c.stops = c.stopsFn(c.r, c.view)
	}
	return c.stops
}

// Bound is the document position a caret stands for as a range's end
// (Stops.Bound).
func (c *Ctx) Bound(caret doc.Caret) doc.Pos { return c.Stops().Bound(caret) }

// Range is what the selection spans, in document order: the bounds of a
// text selection's ends, the gaps around a selected node, or a cell
// range's first cell's start and last cell's end.
func (c *Ctx) Range() (from, to doc.Pos) {
	return SelectionRange(c.r, c.Stops(), c.Selection())
}

// Piece is one piece of what a selection covers, between two positions.
type Piece struct{ From, To doc.Pos }

// Pieces are what the selection covers, in reading order: its extent
// (vocab.SelectionExtent) bound to positions, one piece for a range or a
// selected node, one per cell for a cell range. Every edit of the
// selection reads them, so what is shown selected is what is edited.
func (c *Ctx) Pieces() []Piece { return SelectionPieces(c.r, c.Stops(), c.Selection()) }

// SelectionPieces are a selection's pieces over a stop model, as Pieces
// reads them.
func SelectionPieces(r doc.Reader, stops Stops, sel doc.Selection) []Piece {
	if sel.Kind != doc.SelectCells {
		if sel.Kind == doc.SelectText && sel.IsCollapsed() {
			return nil
		}
		from, to := SelectionRange(r, stops, sel)
		return []Piece{{from, to}}
	}
	var out []Piece
	for _, e := range vocab.SelectionExtent(r, sel) {
		out = append(out, Piece{bound(stops, e.From), bound(stops, e.To)})
	}
	return out
}

func bound(stops Stops, c doc.Caret) doc.Pos {
	if stops == nil {
		return c.Pos
	}
	return stops.Bound(c)
}

// SelectionRange is a selection's range over a stop model, as Range reads
// it; the engine publishes the same range as the selection's coverage.
func SelectionRange(r doc.Reader, stops Stops, sel doc.Selection) (from, to doc.Pos) {
	if sel.Kind == doc.SelectNode {
		if parent, slot, index, ok := vocab.Index(r, sel.Node); ok {
			return doc.Pos{Node: parent, Slot: slot, Offset: index}, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}
		}
	}
	if sel.Kind == doc.SelectCells {
		if pieces := SelectionPieces(r, stops, sel); len(pieces) > 0 {
			return pieces[0].From, pieces[len(pieces)-1].To
		}
	}
	a, b := sel.Ordered(r)
	return stops.Bound(a), stops.Bound(b)
}

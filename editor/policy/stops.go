package policy

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Stops is a view's stop model: the caret stops the presentation defines,
// in logical order. The engine owns it and builds it from the presentation,
// so logical motion in policies lands exactly where the renderer shows
// carets. Entry stops, inside attached surfaces, are reached only through
// explicit intents; ordinary motion moves among them only while the caret
// is already in their surface, and leaves through the surface's edges.
type Stops interface {
	// Step is the stop next to c in dir, or false at the page's edge.
	Step(c doc.Caret, dir vocab.Direction) (doc.Caret, bool)
	// Edge is the page's first stop (Backward) or last stop (Forward).
	Edge(dir vocab.Direction) (doc.Caret, bool)
	// Normalize is the valid stop nearest c: c itself when it is a stop.
	Normalize(c doc.Caret) doc.Caret
	// Bound is the document position c stands for as the end of a range:
	// its own position, except that the stop before a block's marker
	// stands before the block, at the gap in the slot holding it.
	Bound(c doc.Caret) doc.Pos
}

// StopsFunc builds the stop model of a document as a view sees it.
type StopsFunc func(r doc.Reader, view *View) Stops

// WithStops returns a context whose Stops come from fn.
func (c *Ctx) WithStops(fn StopsFunc) *Ctx {
	out := *c
	out.stopsFn = fn
	out.stops = nil
	return &out
}

// Stops is the stop model of the context's document and view. It is nil
// for a context the engine did not build, such as a bare dispatch test.
func (c *Ctx) Stops() Stops {
	if c.stops == nil && c.stopsFn != nil {
		c.stops = c.stopsFn(c.r, c.view)
	}
	return c.stops
}

// Bound is the document position a caret stands for as a range's end
// (Stops.Bound); without a stop model, its own position.
func (c *Ctx) Bound(caret doc.Caret) doc.Pos {
	if st := c.Stops(); st != nil {
		return st.Bound(caret)
	}
	return caret.Pos
}

// Range is what the selection covers, in document order: the bounds of a
// text selection's ends, or the gaps around a selected node. Every edit of
// the selection reads it, so what is shown selected is what is edited.
func (c *Ctx) Range() (from, to doc.Pos) {
	return SelectionRange(c.r, c.Stops(), c.Selection())
}

// SelectionRange is a selection's range over a stop model, as Range reads
// it; the engine publishes the same range as the selection's coverage.
func SelectionRange(r doc.Reader, stops Stops, sel doc.Selection) (from, to doc.Pos) {
	if sel.Kind == doc.SelectNode {
		if parent, slot, index, ok := vocab.Index(r, sel.Node); ok {
			return doc.Pos{Node: parent, Slot: slot, Offset: index}, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}
		}
	}
	a, b := sel.Ordered(r)
	if stops == nil {
		return a.Pos, b.Pos
	}
	return stops.Bound(a), stops.Bound(b)
}

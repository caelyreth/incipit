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

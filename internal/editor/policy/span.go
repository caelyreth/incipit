package policy

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// A selection's span is what it covers, recognised once for every
// reader: the rules acting on a range, the conditions offering commands
// (offers.go), and the hosts asking what applies. A rule never works the
// blocks a range touches out again, and a host never guesses what is
// selected from the document's kinds.

// SpanShape is a set of the shapes a selection's span takes.
type SpanShape uint16

const (
	// SpanCaret is a collapsed caret.
	SpanCaret SpanShape = 1 << iota
	// SpanWord is a range over exactly one word of a slot's text
	// (vocab.Words), SpanSentence one sentence (vocab.Sentences), and
	// SpanText any other part of one slot's text short of all of it.
	SpanWord
	SpanSentence
	SpanText
	// SpanBlocks is a range over all of a block's text, or across
	// blocks.
	SpanBlocks
	// SpanNode is a node selected whole.
	SpanNode
	// SpanCells is a range of a table's cells.
	SpanCells

	// SpanInline is a part of one slot's text.
	SpanInline = SpanWord | SpanSentence | SpanText
	// SpanWriting is wherever text is written: a caret, a part of a
	// slot's text, or blocks.
	SpanWriting = SpanCaret | SpanInline | SpanBlocks
	// SpanAny is every shape.
	SpanAny = SpanWriting | SpanNode | SpanCells
)

// SelectionSpan is what a selection spans.
type SelectionSpan struct {
	Shape SpanShape
	// From and To bound it, as Range does.
	From, To doc.Pos
	// Blocks are the text blocks it touches, in reading order: a caret's
	// one, a range's every one, the text blocks of the cells of a cell
	// range; a node selected, where it is one.
	Blocks []*doc.Node
	// Rows and Columns are set on a cell range of whole rows or whole
	// columns.
	Rows, Columns bool
}

// SpanOf is what sel spans over a stop model.
func SpanOf(r doc.Reader, stops Stops, sel doc.Selection) SelectionSpan {
	from, to := SelectionRange(r, stops, sel)
	e := SelectionSpan{From: from, To: to}
	switch {
	case sel.Kind == doc.SelectNode:
		e.Shape = SpanNode
		if n, ok := r.Node(sel.Node); ok && r.Schema().IsTextBlock(n.Kind()) {
			e.Blocks = []*doc.Node{n}
		}
		return e
	case sel.Kind == doc.SelectCells:
		e.Shape = SpanCells
		e.Rows, e.Columns = sel.Cells.WholeRows, sel.Cells.WholeColumns
		for _, p := range SelectionPieces(r, stops, sel) {
			e.Blocks = appendBlocks(e.Blocks, vocab.SelectedBlocks(r, p.From, p.To))
		}
		return e
	case sel.IsCollapsed():
		e.Shape = SpanCaret
	}
	e.Blocks = vocab.SelectedBlocks(r, from, to)
	if e.Shape == SpanCaret {
		return e
	}
	e.Shape = inlineShape(r, from, to)
	return e
}

// inlineShape is the shape of a range from from to to: a part of one
// slot's text, as a word or a sentence of it where it is exactly one;
// else blocks.
func inlineShape(r doc.Reader, from, to doc.Pos) SpanShape {
	if from.Ref() != to.Ref() {
		return SpanBlocks
	}
	n, ok := r.Node(from.Node)
	if !ok || from.Slot >= n.NumSlots() || !n.Slot(from.Slot).IsInline() {
		return SpanBlocks
	}
	in := n.Slot(from.Slot).Content()
	if from.Offset <= 0 && to.Offset >= in.Len() {
		return SpanBlocks
	}
	for _, w := range vocab.Words(in) {
		if w.Word && w.From == from.Offset && w.To == to.Offset {
			return SpanWord
		}
	}
	if s, ok := vocab.SentenceAround(in, from.Offset, to.Offset); ok && s.From == from.Offset && s.To == to.Offset {
		return SpanSentence
	}
	return SpanText
}

func appendBlocks(out, blocks []*doc.Node) []*doc.Node {
	for _, b := range blocks {
		if len(out) == 0 || out[len(out)-1] != b {
			out = append(out, b)
		}
	}
	return out
}

// Span is what the view's selection spans, recognised once for the
// context.
func (c *Ctx) Span() *SelectionSpan {
	if c.span == nil || c.spanOf != c.view.Selection {
		e := SpanOf(c.r, c.Stops(), c.view.Selection)
		c.span, c.spanOf = &e, c.view.Selection
	}
	return c.span
}

// Spans holds where the selection's span takes one of shapes.
func Spans(shapes SpanShape) Cond {
	return Cond{test: func(c *Ctx) bool { return c.Span().Shape&shapes != 0 }}
}

// WholeRows holds where whole rows of a table are selected, WholeColumns
// whole columns.
var (
	WholeRows    = Cond{test: func(c *Ctx) bool { return c.Span().Rows }}
	WholeColumns = Cond{test: func(c *Ctx) bool { return c.Span().Columns }}
)

// EveryBlockIn holds where the span touches text blocks, each of them
// of kind k or inside a node of kind k: every one of the items selected,
// say, in a list.
func EveryBlockIn(k doc.Kind) Cond {
	return Cond{test: func(c *Ctx) bool {
		blocks := c.Span().Blocks
		for _, b := range blocks {
			if _, ok := Within(c.Doc(), b, k); !ok {
				return false
			}
		}
		return len(blocks) > 0
	}}
}

// Within is the nearest node of kind k holding n, n itself included.
func Within(r doc.Reader, n *doc.Node, k doc.Kind) (*doc.Node, bool) {
	for id := n.ID(); ; {
		node, ok := r.Node(id)
		if !ok {
			return nil, false
		}
		if node.Kind() == k {
			return node, true
		}
		parent, _, ok := r.Parent(id)
		if !ok {
			return nil, false
		}
		id = parent
	}
}

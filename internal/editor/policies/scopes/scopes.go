// Package scopes is the scopes policy: SelectScope grows from the
// innermost scope at the caret through each enclosing scope, and
// ExpandSelection and ShrinkSelection climb and come back down the whole
// ladder, the word and the sentence among its rungs (ladder.go). Escape in a
// scope one leaves, with nothing selected, selects it whole when it is a
// node; on it selected whole, it enters it again where Return would.
package scopes

import (
	"slices"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "scopes"

// Policy is the scopes policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	policy.On(p, "select", policy.Intent[vocab.SelectScope](), policy.Slot.Inside(), grow).Any()
	policy.On(p, "expand", policy.Intent[vocab.ExpandSelection](), policy.Slot.Inside(), expand).Any()
	policy.On(p, "shrink", policy.Intent[vocab.ShrinkSelection](), policy.Slot.Inside(), shrink).Any()
	writing := policy.Spans(policy.SpanWriting)
	p.Command("scopes.expand", "commands.expandSelection", vocab.ExpandSelection{}, policy.Key("alt+shift+primary+right"),
		policy.Offer(vocab.SelectionOffers, 1, "right", writing))
	p.Command("scopes.shrink", "commands.shrinkSelection", vocab.ShrinkSelection{}, policy.Key("alt+shift+primary+left"),
		policy.Offer(vocab.SelectionOffers, 0, "left", writing))
	leavable := policy.Role(vocab.RoleLeavable)
	policy.On(p, "leave-selecting", policy.Intent[vocab.Dismiss](), leavable.Nodes().Own(), selectScope).Caret()
	policy.On(p, "enter-selected", policy.Intent[vocab.Dismiss](), leavable.Selected(), enterScope).Node()
	// The innermost textual scope at a collapsed caret.
	p.Paint(vocab.GroupScope, leavable.Facts().Inside())

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Grow (from the inside"),
		policy.Waits(500*time.Millisecond),
		policy.Presses(vocab.SelectScope{}),
		policy.Waits(600*time.Millisecond),
		policy.Presses(vocab.SelectScope{}),
	)
}, policy.Label("policies.scopes"), policy.Description("policies.scopes.description"))

type candidate struct {
	selection doc.Selection
}

// grow selects the innermost scope containing the current selection, or
// the next enclosing scope when the selection already is a scope, and
// keeps the rung it climbed, for ShrinkSelection to come back down. The
// scope stack is read at both stands of the selection: selecting a mark
// or pair can leave its head outside the scope its anchor still holds.
func grow(c *policy.Ctx, _ vocab.SelectScope, _ policy.At[doc.SlotRef]) policy.Result {
	return climb(c, candidatesAt(c))
}

// climb selects the rung of cands, outer first, next around the
// selection: from a caret the innermost holding it, from a rung the one
// around it, else the innermost holding the selection.
func climb(c *policy.Ctx, cands []candidate) policy.Result {
	sel := c.Selection()
	next, ok := nextOf(c, cands, sel)
	if !ok {
		return policy.Swallow
	}
	res := policy.Select(next)
	res.Mode = &rung{at: next, from: sel, below: climbed(c, sel)}
	return res
}

// nextOf is the rung of cands, outer first, next around sel.
func nextOf(c *policy.Ctx, cands []candidate, sel doc.Selection) (doc.Selection, bool) {
	if sel.Kind == doc.SelectText && sel.IsCollapsed() {
		for i := len(cands) - 1; i >= 0; i-- {
			if containsCaret(c, cands[i], sel.Head) {
				return cands[i].selection, true
			}
		}
		return doc.Selection{}, false
	}
	for i, cand := range cands {
		if sameSelection(c, cand.selection, sel) {
			for j := i - 1; j >= 0; j-- {
				if containsSelection(c, cands[j], sel) && !sameSelection(c, cands[j].selection, sel) {
					return cands[j].selection, true
				}
			}
			return doc.Selection{}, false
		}
	}
	for i := len(cands) - 1; i >= 0; i-- {
		if containsSelection(c, cands[i], sel) {
			return cands[i].selection, true
		}
	}
	return doc.Selection{}, false
}

// selectScope selects a node scope whole from inside it.
func selectScope(c *policy.Ctx, _ vocab.Dismiss, at policy.At[vocab.Leavable]) policy.Result {
	if c.Composition() != nil {
		return policy.Pass
	}
	s, ok := selectionOf(c, at.Level)
	if !ok {
		return policy.Pass
	}
	return policy.Select(settled(c, s))
}

// enterScope goes back into a scope selected whole, at its entry.
func enterScope(c *policy.Ctx, _ vocab.Dismiss, at policy.At[vocab.Leavable]) policy.Result {
	return policy.Select(doc.Collapsed(at.Data.Entry()))
}

// candidatesAt returns the scopes the two ends of the selection stand in,
// outer first. A candidate with no content is not selectable.
func candidatesAt(c *policy.Ctx) []candidate {
	sel := c.Selection()
	ends := []doc.Caret{sel.Anchor}
	if sel.Kind != doc.SelectText {
		// An object selected stands in the scopes around its place, a cell
		// range in those of its head cell.
		ends = []doc.Caret{c.Head()}
	} else if sel.Head != sel.Anchor {
		ends = append(ends, sel.Head)
	}
	var out []candidate
	for _, end := range ends {
		for _, level := range stackAt(c, end).Levels {
			if !presented(c.Doc(), level) {
				continue
			}
			s, ok := selectionOf(c, level)
			if !ok {
				continue
			}
			s = settled(c, s)
			from, to := policy.SelectionRange(c.Doc(), c.Stops(), s)
			if from == to {
				continue
			}
			cand := candidate{selection: s}
			if !slices.ContainsFunc(out, func(other candidate) bool {
				return sameSelection(c, other.selection, cand.selection)
			}) {
				out = append(out, cand)
			}
		}
	}
	order(c, out)
	return out
}

// order sorts cands outer first: each before those it holds, those apart
// in reading order.
func order(c *policy.Ctx, cands []candidate) {
	slices.SortStableFunc(cands, func(a, b candidate) int {
		ab, ba := containsSelection(c, a, b.selection), containsSelection(c, b, a.selection)
		switch {
		case ab && ba:
			// The same extent, as a page holding only a table and the
			// table: in the stack's order, outer first.
			return 0
		case ab:
			return -1
		case ba:
			return 1
		}
		af, at := extent(c, a.selection)
		bf, bt := extent(c, b.selection)
		if n := doc.Compare(c.Doc(), af, bf); n != 0 {
			return n
		}
		return doc.Compare(c.Doc(), bt, at)
	})
}

// presented reports whether a level is a place the page presents, which
// a scope can be: anything outside an object; inside one, its text slots,
// what stands in their text, and the object itself. An object is one unit
// whose text, however deep, is its surfaces; the nodes and block slots
// between it and its text only arrange them, as a table's rows its cells,
// so they are no scopes.
func presented(r doc.Reader, l *policy.Level) bool {
	switch l.Class {
	case policy.LevelNode:
		if l.Node == nil {
			return false
		}
		obj, in := vocab.ObjectOf(r, l.Node.ID())
		return !in || obj == l.Node.ID()
	case policy.LevelSlot:
		n := l.Node
		if n == nil || l.Slot.Slot >= len(n.Slots()) {
			return false
		}
		if n.Slots()[l.Slot.Slot].IsInline() {
			return true
		}
		_, in := vocab.ObjectOf(r, n.ID())
		return !in
	}
	return true
}

// settled puts a candidate's ends on stops and shapes its range, as the
// engine settles every selection it publishes: a row's text is the cell
// range of its cells.
func settled(c *policy.Ctx, s doc.Selection) doc.Selection {
	if s.Kind != doc.SelectText {
		return s
	}
	stops := c.Stops()
	s.Anchor = stops.Normalize(s.Anchor)
	s.Head = stops.Normalize(s.Head)
	return vocab.ShapeSelection(c.Doc(), c.Program().Profile(), s)
}

// stackAt reads the scope stack at a caret without publishing a view.
func stackAt(c *policy.Ctx, at doc.Caret) *policy.Stack {
	v := *c.View()
	v.Selection = doc.Collapsed(at)
	return c.Over(c.Doc(), &v).Stack()
}

// selectionOf is the selection a stack level denotes. A node and a slot
// hold all their block structure, while a fact or annotation holds the
// content between its inner edges.
func selectionOf(c *policy.Ctx, level *policy.Level) (doc.Selection, bool) {
	switch level.Class {
	case policy.LevelNode:
		if level.Node == nil {
			return doc.Selection{}, false
		}
		if vocab.SelectedWhole(c.Program().Profile(), level.Node.Kind()) {
			return doc.NodeSelection(level.Node.ID()), true
		}
		return boundarySelection(c, level.Start, level.End), true
	case policy.LevelSlot:
		if s, ok := textOf(c, level); ok {
			return s, true
		}
		return boundarySelection(c, level.Start, level.End), true
	case policy.LevelFact, policy.LevelAnnotation:
		return doc.TextRange(level.Start, level.End), level.Start != level.End
	}
	return doc.Selection{}, false
}

// textOf is what source text's slot denotes, as a code block's: its own
// text, from its first stop to its last, the fences its block spells
// around it left out, so the code is selected, then the block whole (its
// node). Prose's slot holds its block's marker: a heading is selected with
// its hashes.
func textOf(c *policy.Ctx, level *policy.Level) (doc.Selection, bool) {
	spec, ok := vocab.SlotSpec(c.Doc(), level.Slot)
	in, ok2 := c.Inline(level.Slot)
	if !ok || !ok2 || !spec.Literal || !spec.Lines {
		return doc.Selection{}, false
	}
	return doc.TextRange(doc.Caret{Pos: doc.At(level.Slot, 0)}, doc.Caret{Pos: doc.At(level.Slot, in.Len())}), in.Len() > 0
}

// boundarySelection takes every stop around a level's structural edges,
// so a block's marker is held with the block.
func boundarySelection(c *policy.Ctx, start, end doc.Caret) doc.Selection {
	first, _ := c.Stops().Around(start.Pos)
	_, last := c.Stops().Around(end.Pos)
	start.Stop, end.Stop = first, last
	return doc.TextRange(start, end)
}

// extent is a selection's two ends as places, in reading order.
func extent(c *policy.Ctx, s doc.Selection) (from, to doc.Caret) {
	r := c.Doc()
	if s.Kind == doc.SelectText {
		return s.Ordered(r)
	}
	if n, ok := r.Node(s.Node); ok && s.Kind == doc.SelectNode && !r.Schema().IsObject(n.Kind()) {
		// A text block selected whole, as code with its fences, stands
		// across every stop of its text's edges, as its level does: so a
		// range from before its notation holds it, and it the text inside.
		first, ok1 := vocab.FirstTextSlot(n)
		last, ok2 := vocab.LastTextSlot(n)
		if ok1 && ok2 {
			sel := boundarySelection(c, doc.CaretAt(doc.At(first, 0)), doc.CaretAt(doc.At(last, n.Slot(last.Slot).Len())))
			return sel.Anchor, sel.Head
		}
	}
	fromPos, toPos := policy.SelectionRange(r, nil, s)
	return doc.CaretAt(fromPos), doc.CaretAt(toPos)
}

func containsCaret(c *policy.Ctx, cand candidate, at doc.Caret) bool {
	from, to := extent(c, cand.selection)
	return doc.Compare(c.Doc(), from, at) <= 0 && doc.Compare(c.Doc(), at, to) <= 0
}

func containsSelection(c *policy.Ctx, outer candidate, inner doc.Selection) bool {
	of, ot := extent(c, outer.selection)
	innerFrom, innerTo := extent(c, inner)
	return doc.Compare(c.Doc(), of, innerFrom) <= 0 && doc.Compare(c.Doc(), innerTo, ot) <= 0
}

func sameSelection(c *policy.Ctx, a, b doc.Selection) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == doc.SelectNode {
		return a.Node == b.Node
	}
	af, at := extent(c, a)
	bf, bt := extent(c, b)
	return af == bf && at == bt
}

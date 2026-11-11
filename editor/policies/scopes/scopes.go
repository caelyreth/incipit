// Package scopes is the scopes policy: SelectScope grows from the
// innermost scope at the caret through each enclosing scope. Dismiss is
// core's.
package scopes

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "scopes"

// Active marks the innermost textual scope at a collapsed caret.
var Active = policy.NewAnnotationKind("scopes.active", false)

// Policy is the scopes policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	policy.On(p, "select", policy.Intent[vocab.SelectScope](), policy.Slot.Inside(), grow).Any()
	p.Annotation(Active, valid)
	p.Decorate(Active, vocab.GroupScope)
	p.Observe("highlight", policy.Trigger{}, highlight)
})

type candidate struct {
	selection doc.Selection
}

// grow selects the innermost scope containing the current selection, or
// the next enclosing scope when the selection already is a scope. The
// scope stack is read at both stands of the selection: selecting a mark
// or pair can leave its head outside the scope its anchor still holds.
func grow(c *policy.Ctx, _ vocab.SelectScope, _ policy.At[doc.SlotRef]) policy.Result {
	sel := c.Selection()
	if sel.Kind != doc.SelectText {
		return policy.Swallow
	}
	cands := candidatesAt(c)
	if len(cands) == 0 {
		return policy.Swallow
	}
	if sel.IsCollapsed() {
		for i := len(cands) - 1; i >= 0; i-- {
			if containsCaret(c.Doc(), cands[i], sel.Head) {
				return policy.Select(cands[i].selection)
			}
		}
		return policy.Swallow
	}
	for i, cand := range cands {
		if sameSelection(c.Doc(), cand.selection, sel) {
			for j := i - 1; j >= 0; j-- {
				if containsSelection(c.Doc(), cands[j], sel) && !sameSelection(c.Doc(), cands[j].selection, sel) {
					return policy.Select(cands[j].selection)
				}
			}
			return policy.Swallow
		}
	}
	for i := len(cands) - 1; i >= 0; i-- {
		if containsSelection(c.Doc(), cands[i], sel) {
			return policy.Select(cands[i].selection)
		}
	}
	return policy.Swallow
}

// highlight keeps one decoration over the innermost fact or scoped
// annotation holding a collapsed caret. A selection already shows its
// own extent, and a point scope has no text to underline.
func highlight(c *policy.Ctx, _ []doc.Record) policy.Result {
	res := policy.Result{DropKinds: []policy.AnnotationKind{Active}}
	sel := c.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		return res
	}
	levels := c.Stack().Levels
	for i := len(levels) - 1; i >= 0; i-- {
		level := levels[i]
		if level.Class != policy.LevelFact && level.Class != policy.LevelAnnotation {
			continue
		}
		if level.Class == policy.LevelFact && !level.Provides(vocab.RoleLeavable) {
			continue
		}
		anchor := policy.Anchor{From: level.Start, To: level.End}
		if anchor.From.Pos == anchor.To.Pos || anchor.From.Pos.Ref() != anchor.To.Pos.Ref() {
			continue
		}
		res.Annotate = []policy.Annotate{{Kind: Active, Anchor: anchor, Lifetime: policy.WithView}}
		return res
	}
	return res
}

// valid keeps the highlight while its span exists.
func valid(r doc.Reader, a policy.Annotation) bool {
	n, ok := r.Node(a.Anchor.From.Pos.Node)
	if !ok || a.Anchor.From.Pos.Ref() != a.Anchor.To.Pos.Ref() {
		return false
	}
	slot := a.Anchor.From.Pos.Slot
	if slot < 0 || slot >= n.NumSlots() {
		return false
	}
	end := n.Slot(slot).Len()
	return a.Anchor.From.Offset >= 0 && a.Anchor.From.Offset <= end &&
		a.Anchor.To.Offset >= a.Anchor.From.Offset && a.Anchor.To.Offset <= end
}

// candidatesAt returns the scopes the two ends of the selection stand in,
// outer first. A candidate with no content is not selectable.
func candidatesAt(c *policy.Ctx) []candidate {
	sel := c.Selection()
	ends := []doc.Caret{sel.Anchor}
	if sel.Head != sel.Anchor {
		ends = append(ends, sel.Head)
	}
	var out []candidate
	for _, end := range ends {
		for _, level := range stackAt(c, end).Levels {
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
				return sameSelection(c.Doc(), other.selection, cand.selection)
			}) {
				out = append(out, cand)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b candidate) int {
		switch {
		case sameSelection(c.Doc(), a.selection, b.selection):
			return 0
		case containsSelection(c.Doc(), a, b.selection):
			return -1
		case containsSelection(c.Doc(), b, a.selection):
			return 1
		}
		af, at := extent(c.Doc(), a.selection)
		bf, bt := extent(c.Doc(), b.selection)
		if n := doc.Compare(c.Doc(), af, bf); n != 0 {
			return n
		}
		return doc.Compare(c.Doc(), bt, at)
	})
	return out
}

// settled puts a candidate's ends on stops, as the engine settles every
// selection it publishes.
func settled(c *policy.Ctx, s doc.Selection) doc.Selection {
	if s.Kind != doc.SelectText {
		return s
	}
	if stops := c.Stops(); stops != nil {
		s.Anchor = stops.Normalize(s.Anchor)
		s.Head = stops.Normalize(s.Head)
	}
	return s
}

// stackAt reads the scope stack at a caret without publishing a view.
func stackAt(c *policy.Ctx, at doc.Caret) *policy.Stack {
	v := *c.View()
	v.Selection = doc.Collapsed(at)
	return policy.NewCtx(c.Program(), c.Store(), c.Doc(), &v, c.Input()).Stack()
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
		if c.Doc().Schema().IsObject(level.Node.Kind()) {
			return doc.NodeSelection(level.Node.ID()), true
		}
		return boundarySelection(c, level.Start, level.End), true
	case policy.LevelSlot:
		return boundarySelection(c, level.Start, level.End), true
	case policy.LevelFact, policy.LevelAnnotation:
		return doc.TextRange(level.Start, level.End), level.Start != level.End
	}
	return doc.Selection{}, false
}

// boundarySelection takes every stop around a level's structural edges,
// so a block's marker is held with the block.
func boundarySelection(c *policy.Ctx, start, end doc.Caret) doc.Selection {
	if stops := c.Stops(); stops != nil {
		first, _ := stops.Around(start.Pos)
		_, last := stops.Around(end.Pos)
		start.Stop, end.Stop = first, last
	}
	return doc.TextRange(start, end)
}

// extent is a selection's two ends as places, in reading order.
func extent(r doc.Reader, s doc.Selection) (from, to doc.Caret) {
	if s.Kind == doc.SelectText {
		return s.Ordered(r)
	}
	fromPos, toPos := policy.SelectionRange(r, nil, s)
	return doc.CaretAt(fromPos), doc.CaretAt(toPos)
}

func containsCaret(r doc.Reader, cand candidate, c doc.Caret) bool {
	from, to := extent(r, cand.selection)
	return doc.Compare(r, from, c) <= 0 && doc.Compare(r, c, to) <= 0
}

func containsSelection(r doc.Reader, outer candidate, inner doc.Selection) bool {
	of, ot := extent(r, outer.selection)
	innerFrom, innerTo := extent(r, inner)
	return doc.Compare(r, of, innerFrom) <= 0 && doc.Compare(r, innerTo, ot) <= 0
}

func sameSelection(r doc.Reader, a, b doc.Selection) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == doc.SelectNode {
		return a.Node == b.Node
	}
	af, at := extent(r, a)
	bf, bt := extent(r, b)
	return af == bf && at == bt
}

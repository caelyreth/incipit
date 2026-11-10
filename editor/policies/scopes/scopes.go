// Package scopes is the scopes policy: SelectScope grows from the
// innermost scope at the caret through each enclosing scope, and Dismiss
// leaves the current selection for its head.
package scopes

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "scopes"

// Policy is the scopes policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Handle("select").On(vocab.On(vocab.IntentSelectScope)).In(vocab.Slot).Do(grow)
	p.Handle("dismiss").On(vocab.On(vocab.IntentDismiss)).In(vocab.Slot).Do(dismiss)
})

type candidate struct {
	selection doc.Selection
	from, to  doc.Pos
}

// grow selects the innermost scope containing the current selection, or
// the next enclosing scope when the selection already is a scope. The
// scope stack is read at both stands of the selection: selecting a mark
// or pair can leave its head outside the scope its anchor still holds.
func grow(c *policy.Ctx, _ policy.Match) policy.Result {
	sel := c.Selection()
	if sel.Kind != doc.SelectText {
		return policy.Swallow
	}
	cands := candidatesAt(c)
	if len(cands) == 0 {
		return policy.Swallow
	}
	curFrom, curTo := c.Range()
	if sel.IsCollapsed() {
		for i := len(cands) - 1; i >= 0; i-- {
			if contains(c.Doc(), cands[i], curFrom, curTo) {
				return policy.Select(cands[i].selection)
			}
		}
		return policy.Swallow
	}
	for i, cand := range cands {
		if sameRange(cand, curFrom, curTo) {
			for j := i - 1; j >= 0; j-- {
				if contains(c.Doc(), cands[j], curFrom, curTo) && !sameRange(cands[j], curFrom, curTo) {
					return policy.Select(cands[j].selection)
				}
			}
			return policy.Swallow
		}
	}
	for i := len(cands) - 1; i >= 0; i-- {
		if contains(c.Doc(), cands[i], curFrom, curTo) {
			return policy.Select(cands[i].selection)
		}
	}
	return policy.Swallow
}

// dismiss collapses a text range at its head and ends a composition.
func dismiss(c *policy.Ctx, _ policy.Match) policy.Result {
	res := policy.Result{}
	if c.Composition() != nil {
		res.EndComposition = true
	}
	sel := c.Selection()
	if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		s := doc.Collapsed(c.Head())
		res.Selection = &s
	}
	return res
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
			from, to := policy.SelectionRange(c.Doc(), c.Stops(), s)
			if from == to {
				continue
			}
			cand := candidate{selection: s, from: from, to: to}
			if !slices.ContainsFunc(out, func(c candidate) bool {
				return c.from == cand.from && c.to == cand.to
			}) {
				out = append(out, cand)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b candidate) int {
		switch {
		case sameRange(a, b.from, b.to):
			return 0
		case contains(c.Doc(), a, b.from, b.to):
			return -1
		case contains(c.Doc(), b, a.from, a.to):
			return 1
		}
		if n := doc.Compare(c.Doc(), doc.CaretAt(a.from), doc.CaretAt(b.from)); n != 0 {
			return n
		}
		return doc.Compare(c.Doc(), doc.CaretAt(b.to), doc.CaretAt(a.to))
	})
	return out
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
		if vocab.IsObject(level.Node.Kind()) {
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

func contains(r doc.Reader, cand candidate, from, to doc.Pos) bool {
	return doc.Compare(r, doc.CaretAt(cand.from), doc.CaretAt(from)) <= 0 &&
		doc.Compare(r, doc.CaretAt(cand.to), doc.CaretAt(to)) >= 0
}

func sameRange(c candidate, from, to doc.Pos) bool {
	return c.from == from && c.to == to
}

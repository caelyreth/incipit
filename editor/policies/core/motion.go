package core

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Logical motion moves through the stop model the engine provides (§13.5):
// grapheme steps and page edges go stop by stop, so the caret lands where
// the presentation shows stops, ghost notation and gaps included.
// Line-edge and slot-edge motion resolves its target in the slot's text
// and continues through the stop model when that target is the slot's
// edge; the words policy resolves words, sentences and paragraphs.

// slotTarget is where one unit of motion from pos lands inside its inline
// slot; false when pos is at the slot's edge in dir, or not in text.
func slotTarget(r doc.Reader, pos doc.Pos, unit vocab.Unit, dir vocab.Direction) (doc.Pos, bool) {
	n, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= n.NumSlots() || !n.Slot(pos.Slot).IsInline() {
		return pos, false
	}
	in := n.Slot(pos.Slot).Content()
	if (dir > 0 && pos.Offset >= in.Len()) || (dir < 0 && pos.Offset <= 0) {
		return pos, false
	}
	switch unit {
	case vocab.ToSlotEdge:
		// ToSlotEdge stops at the line's edge, or the slot's edge when it is
		// already there.
		if dir > 0 {
			end := in.Len()
			if i := indexByte(in.Flat(), '\n', pos.Offset); i >= 0 {
				end = i
			}
			if end == pos.Offset {
				end = in.Len()
			}
			return doc.At(pos.Ref(), end), true
		}
		start := 0
		if i := lastIndexByte(in.Flat()[:pos.Offset], '\n'); i >= 0 {
			start = i + 1
		}
		if start == pos.Offset {
			start = 0
		}
		return doc.At(pos.Ref(), start), true
	}
	// The words policy answers word, sentence and paragraph motion. Core's
	// fallback remains a grapheme step so it answers every intent even
	// when that policy is absent.
	off, _ := vocab.GraphemeStep(in.Flat(), pos.Offset, dir)
	return doc.At(pos.Ref(), off), true
}

func indexByte(s string, b byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// collapseRange collapses a range to its edge in the direction of
// motion, whatever the unit: a range is where a move starts from, and the
// unit counts from there on the next press. A page edge is no place
// relative to the selection, so motion to it moves on.
func collapseRange(c *policy.Ctx, in vocab.Move, _ policy.At[doc.SlotRef]) policy.Result {
	if in.Unit == vocab.ToPageEdge {
		return policy.Pass
	}
	from, to := c.Selection().Ordered(c.Doc())
	if in.Dir > 0 {
		return policy.Reveal(doc.Collapsed(to))
	}
	return policy.Reveal(doc.Collapsed(from))
}

// leaveObject moves from a selected object to the gap on that side of it.
func leaveObject(c *policy.Ctx, in vocab.Move, _ policy.At[doc.SlotRef]) policy.Result {
	stops := c.Stops()
	if stops == nil {
		return policy.Swallow
	}
	gap := c.Head().Pos
	if in.Dir > 0 {
		gap.Offset++
	}
	return policy.Reveal(doc.Collapsed(stops.Normalize(doc.CaretAt(gap))))
}

// move is logical motion of the head: the caret, or the head of the
// range it extends.
func move(c *policy.Ctx, in vocab.Move, _ policy.At[doc.SlotRef]) policy.Result {
	stops := c.Stops()
	if stops == nil {
		return policy.Swallow
	}
	if in.Dir == 0 {
		in.Dir = vocab.Forward
	}
	target, ok := motionTarget(c.Doc(), stops, c.Head(), in.Unit, in.Dir)
	if !ok {
		return policy.Swallow
	}
	if sel := c.Selection(); in.Extend && sel.Kind == doc.SelectText {
		return policy.Reveal(doc.TextRange(sel.Anchor, target))
	}
	return policy.Reveal(doc.Collapsed(target))
}

// motionTarget is where one unit of logical motion from head lands.
func motionTarget(r doc.Reader, stops policy.Stops, head doc.Caret, unit vocab.Unit, dir vocab.Direction) (doc.Caret, bool) {
	switch unit {
	case vocab.ToPageEdge:
		return stops.Edge(dir)
	case vocab.ByGrapheme:
		return stops.Step(head, dir)
	}
	if pos, ok := slotTarget(r, head.Pos, unit, dir); ok {
		return doc.CaretAt(pos), true
	}
	return stops.Step(head, dir)
}

// visualMove accepts the stop the renderer resolved from geometry.
func visualMove(c *policy.Ctx, in vocab.VisualMove, _ policy.At[doc.SlotRef]) policy.Result {
	target := in.Target
	if _, ok := c.Doc().Node(target.Pos.Node); !ok {
		// No line beyond the caret's: a vertical move goes to the page's
		// edge that way.
		dir := vocab.Forward
		switch in.Dir {
		case vocab.Up, vocab.PageUp:
			dir = vocab.Backward
		case vocab.Down, vocab.PageDown:
		default:
			return policy.Swallow
		}
		edge, ok := c.Stops().Edge(dir)
		if !ok {
			return policy.Swallow
		}
		target = edge
	}
	res := policy.Reveal(doc.Collapsed(target))
	if sel := c.Selection(); in.Extend && sel.Kind == doc.SelectText {
		s := doc.TextRange(sel.Anchor, target)
		res.Selection = &s
	}
	if in.Dir == vocab.Up || in.Dir == vocab.Down || in.Dir == vocab.PageUp || in.Dir == vocab.PageDown {
		goal := float64(in.GoalX)
		res.GoalX = &goal
	}
	return res
}

// placeCaret puts the caret where the pointer hit: a stop, an object, a word
// on a double click, the slot on a triple click.
func placeCaret(c *policy.Ctx, in vocab.Place, _ policy.At[doc.SlotRef]) policy.Result {
	r := c.Doc()
	if in.Node != 0 {
		if _, ok := r.Node(in.Node); ok {
			return policy.Select(doc.NodeSelection(in.Node))
		}
	}
	stop := in.Stop
	if _, ok := r.Node(stop.Pos.Node); !ok {
		return policy.Swallow
	}
	sel := c.Selection()
	if in.Extend && sel.Kind == doc.SelectText {
		anchor := sel.Anchor
		if in.Anchor != nil {
			anchor = *in.Anchor
		}
		return policy.Select(doc.TextRange(anchor, stop))
	}
	switch {
	case in.Clicks >= 3:
		n, _ := r.Node(stop.Pos.Node)
		if stop.Pos.Slot < n.NumSlots() && n.Slot(stop.Pos.Slot).IsInline() {
			ref := stop.Pos.Ref()
			return policy.Select(doc.TextRange(doc.CaretAt(doc.At(ref, 0)), doc.CaretAt(doc.At(ref, n.Slot(stop.Pos.Slot).Len()))))
		}
	}
	return policy.Select(doc.Collapsed(stop))
}

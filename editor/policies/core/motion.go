package core

import (
	"unicode/utf8"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Logical motion moves through the stop model the engine provides (§13.5):
// grapheme steps and page edges go stop by stop, so the caret lands where
// the presentation shows stops, ghost notation and gaps included. Word,
// line and slot motion find their target in the slot's text and step
// through the stop model at the slot's edges.

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
	case vocab.ByWord:
		return wordTarget(in, pos, dir)
	case vocab.ToSlotEdge, vocab.BySentence, vocab.ByParagraph:
		// TODO(words): the words policy answers sentences and paragraphs
		// with UAX #29 boundaries; core's are the slot, and ToSlotEdge
		// stops at the line's edge.
		lineEdge := unit == vocab.ToSlotEdge
		if dir > 0 {
			end := in.Len()
			if lineEdge {
				if i := indexByte(in.Text(), '\n', pos.Offset); i >= 0 {
					end = i
				}
			}
			if end == pos.Offset {
				end = in.Len()
			}
			return doc.At(pos.Ref(), end), true
		}
		start := 0
		if lineEdge {
			if i := lastIndexByte(in.Text()[:pos.Offset], '\n'); i >= 0 {
				start = i + 1
			}
		}
		if start == pos.Offset {
			start = 0
		}
		return doc.At(pos.Ref(), start), true
	}
	if dir > 0 {
		return doc.At(pos.Ref(), nextBoundary(in, pos.Offset)), true
	}
	return doc.At(pos.Ref(), prevBoundary(in, pos.Offset)), true
}

func nextBoundary(in *doc.Inline, off int) int {
	it, start, ok := in.ItemAt(off)
	if !ok || it.Node != nil {
		return off + 1
	}
	return start + vocab.NextGrapheme(it.Text, off-start)
}

func prevBoundary(in *doc.Inline, off int) int {
	it, start, ok := in.ItemAt(off - 1)
	if !ok || it.Node != nil {
		return off - 1
	}
	return start + vocab.PrevGrapheme(it.Text, off-start)
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

// wordTarget skips spaces, then one run of word characters (or a single
// other character); false at the slot's edge in dir.
// TODO(words): the words policy answers ByWord with UAX #29 boundaries
// and CJK dictionaries; this is core's fallback.
func wordTarget(in *doc.Inline, pos doc.Pos, dir vocab.Direction) (doc.Pos, bool) {
	text := in.Text()
	off := pos.Offset
	if (dir > 0 && off == len(text)) || (dir < 0 && off == 0) {
		return pos, false
	}
	class := func(i int) vocab.CharClass {
		var ru rune
		if dir > 0 {
			ru, _ = utf8.DecodeRuneInString(text[i:])
		} else {
			ru, _ = utf8.DecodeLastRuneInString(text[:i])
		}
		return vocab.ClassOf(ru)
	}
	adv := func(i int) int {
		if dir > 0 {
			_, n := utf8.DecodeRuneInString(text[i:])
			return i + n
		}
		_, n := utf8.DecodeLastRuneInString(text[:i])
		return i - n
	}
	more := func(i int) bool { return (dir > 0 && i < len(text)) || (dir < 0 && i > 0) }
	for more(off) && class(off)&vocab.ClassSpace != 0 {
		off = adv(off)
	}
	if more(off) && class(off)&vocab.ClassWord != 0 {
		for more(off) && class(off)&vocab.ClassWord != 0 {
			off = adv(off)
		}
	} else if more(off) {
		off = adv(off)
	}
	return doc.At(pos.Ref(), off), true
}

// move is logical motion; without Extend, a range collapses to its edge
// in the direction of motion first.
func move(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Move)
	r := c.Doc()
	sel := c.Selection()
	stops := c.Stops()
	if stops == nil {
		return policy.Swallow
	}
	if in.Dir == 0 {
		in.Dir = vocab.Forward
	}
	if !in.Extend && sel.Kind == doc.SelectText && !sel.IsCollapsed() && in.Unit == vocab.ByGrapheme {
		from, to := sel.Ordered(r)
		edge := from
		if in.Dir > 0 {
			edge = to
		}
		return revealed(doc.Collapsed(edge))
	}
	if sel.Kind == doc.SelectNode && !in.Extend {
		// Leaving a selected object: to the gap on that side of it.
		gap := c.Head().Pos
		if in.Dir > 0 {
			gap.Offset++
		}
		return revealed(doc.Collapsed(stops.Normalize(doc.CaretAt(gap))))
	}
	head := c.Head()
	target, ok := motionTarget(r, stops, head, in.Unit, in.Dir)
	if !ok {
		return policy.Swallow
	}
	if in.Extend && sel.Kind == doc.SelectText {
		return revealed(doc.TextRange(sel.Anchor, target))
	}
	return revealed(doc.Collapsed(target))
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

func revealed(s doc.Selection) policy.Result {
	return policy.Result{Selection: &s, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// visualMove accepts the stop the renderer resolved from geometry.
func visualMove(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.VisualMove)
	target := in.Target
	if _, ok := c.Doc().Node(target.Pos.Node); !ok {
		return policy.Swallow
	}
	res := revealed(doc.Collapsed(target))
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
func placeCaret(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Place)
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
		return policy.Select(doc.TextRange(sel.Anchor, stop))
	}
	switch {
	case in.Clicks == 2:
		n, _ := r.Node(stop.Pos.Node)
		if stop.Pos.Slot < n.NumSlots() && n.Slot(stop.Pos.Slot).IsInline() {
			in := n.Slot(stop.Pos.Slot).Content()
			at := doc.At(stop.Pos.Ref(), stop.Pos.Offset)
			back, ok := wordTarget(in, at, vocab.Backward)
			if !ok {
				back = at
			}
			fwd, ok := wordTarget(in, back, vocab.Forward)
			if !ok {
				fwd = back
			}
			return policy.Select(doc.TextRange(doc.CaretAt(back), doc.CaretAt(fwd)))
		}
	case in.Clicks >= 3:
		n, _ := r.Node(stop.Pos.Node)
		if stop.Pos.Slot < n.NumSlots() && n.Slot(stop.Pos.Slot).IsInline() {
			ref := stop.Pos.Ref()
			return policy.Select(doc.TextRange(doc.CaretAt(doc.At(ref, 0)), doc.CaretAt(doc.At(ref, n.Slot(stop.Pos.Slot).Len()))))
		}
	}
	return policy.Select(doc.Collapsed(stop))
}

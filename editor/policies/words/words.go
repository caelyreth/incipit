// Package words is the words policy: motion, selection and deletion by
// word, sentence and paragraph. Words use the ICU-tailored UAX #29 data in
// vocab; an inline node is one unit. Core keeps grapheme, line and page
// motion, and remains the fallback when this policy is absent.
package words

import (
	"sort"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "words"

var edits = []doc.RecordKind{
	doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks, doc.RecordNodeChanged,
	doc.RecordNodesInserted, doc.RecordNodesRemoved, doc.RecordNodeMoved, doc.RecordTransferred,
}

// Policy is the words policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	for _, unit := range []struct {
		name string
		unit vocab.Unit
	}{
		{"word", vocab.ByWord},
		{"sentence", vocab.BySentence},
		{"paragraph", vocab.ByParagraph},
	} {
		p.Handle("move-" + unit.name).On(vocab.Moving(unit.unit, 0)).Do(move)
		p.Handle("delete-" + unit.name).On(vocab.Deleting(unit.unit, 0)).Produces(edits...).Do(delete)
	}
	p.Handle("place").On(vocab.On(vocab.IntentPlace)).Do(place)
})

type span struct{ from, to int }

// move moves or extends by the requested unit. A range collapses to its
// edge in the direction of motion first.
func move(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Move)
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Forward
	}
	sel := c.Selection()
	if sel.Kind == doc.SelectNode {
		if in.Extend {
			return policy.Swallow
		}
		stops := c.Stops()
		if stops == nil {
			return policy.Swallow
		}
		gap := c.Head().Pos
		if dir > 0 {
			gap.Offset++
		}
		return revealed(doc.Collapsed(stops.Normalize(doc.CaretAt(gap))))
	}
	if sel.Kind != doc.SelectText {
		return policy.Swallow
	}
	if !in.Extend && !sel.IsCollapsed() {
		from, to := sel.Ordered(c.Doc())
		edge := from
		if dir > 0 {
			edge = to
		}
		return revealed(doc.Collapsed(edge))
	}
	target, ok := targetAt(c, sel.Head, in.Unit, dir)
	if !ok {
		return policy.Swallow
	}
	if in.Extend {
		return revealed(doc.TextRange(sel.Anchor, target))
	}
	return revealed(doc.Collapsed(target))
}

// targetAt resolves one unit of motion from a caret. Inside text its
// target is a text boundary; at a slot's edge the stop model carries it to
// the neighbouring unit.
func targetAt(c *policy.Ctx, head doc.Caret, unit vocab.Unit, dir vocab.Direction) (doc.Caret, bool) {
	in, ok := c.Inline(head.Pos.Ref())
	if ok && head.Pos.Offset >= 0 && head.Pos.Offset <= in.Len() {
		flat := in.Flat()
		if target, found := targetIn(in, flat, head.Pos.Offset, unit, dir); found {
			if target != head.Pos.Offset {
				caret := doc.CaretAt(doc.At(head.Pos.Ref(), target))
				if stops := c.Stops(); stops != nil {
					caret = stops.Normalize(caret)
				}
				return caret, true
			}
			if head.Stop == 0 {
				return head, false
			}
		}
	}
	if stops := c.Stops(); stops != nil {
		return stops.Step(head, dir)
	}
	return head, false
}

func targetIn(in *doc.Inline, flat string, off int, unit vocab.Unit, dir vocab.Direction) (int, bool) {
	switch unit {
	case vocab.ByWord:
		return wordTarget(in, flat, off, dir)
	case vocab.BySentence:
		return sentenceTarget(flat, off, dir)
	case vocab.ByParagraph:
		return paragraphTarget(flat, off, dir)
	}
	return 0, false
}

// wordTarget passes spaces and punctuation to the next word-like segment.
// From inside a word it lands at that word's end; backward it lands at the
// current or previous word's start. Inline nodes are units of their own.
func wordTarget(in *doc.Inline, flat string, off int, dir vocab.Direction) (int, bool) {
	units := wordUnits(in, flat)
	if dir > 0 {
		for _, u := range units {
			if u.to > off {
				return u.to, true
			}
		}
		return graphemeTarget(flat, off, dir)
	}
	for i := len(units) - 1; i >= 0; i-- {
		if units[i].from < off {
			return units[i].from, true
		}
	}
	return graphemeTarget(flat, off, dir)
}

func wordUnits(in *doc.Inline, flat string) []span {
	var units []span
	for segment := range vocab.WordSegments(maskNodes(flat)) {
		if segment.WordLike && segment.Start < len(flat) {
			units = append(units, span{segment.Start, segment.Start + len(segment.Text)})
		}
	}
	in.Nodes(func(at int, _ *doc.Node) bool {
		units = append(units, span{at, at + 1})
		return true
	})
	sort.Slice(units, func(i, j int) bool {
		if units[i].from != units[j].from {
			return units[i].from < units[j].from
		}
		return units[i].to < units[j].to
	})
	return units
}

// sentenceTarget lands at a sentence's end going forward, and at the
// current or previous sentence's start going backward.
func sentenceTarget(flat string, off int, dir vocab.Direction) (int, bool) {
	var units []span
	for segment := range vocab.SentenceSegments(maskNodes(flat)) {
		units = append(units, span{segment.Start, segment.Start + len(segment.Text)})
	}
	if dir > 0 {
		for _, u := range units {
			if u.to > off {
				return u.to, true
			}
		}
	} else {
		for i := len(units) - 1; i >= 0; i-- {
			if units[i].from < off {
				return units[i].from, true
			}
		}
	}
	return graphemeTarget(flat, off, dir)
}

// paragraphTarget moves to the next paragraph's start, or to the current
// paragraph's start and then the previous one. Line separators bound
// paragraphs inside a slot; document paragraphs are separate slots and are
// crossed through the stop model.
func paragraphTarget(flat string, off int, dir vocab.Direction) (int, bool) {
	starts := []int{0}
	for i := 0; i < len(flat); i++ {
		if flat[i] != '\n' {
			continue
		}
		j := i + 1
		for j < len(flat) && (flat[j] == '\n' || flat[j] == '\r') {
			j++
		}
		if j > starts[len(starts)-1] {
			starts = append(starts, j)
		}
		i = j - 1
	}
	if dir > 0 {
		for _, start := range starts {
			if start > off {
				return start, true
			}
		}
		if off < len(flat) {
			return len(flat), true
		}
	} else {
		for i := len(starts) - 1; i >= 0; i-- {
			if starts[i] < off {
				return starts[i], true
			}
		}
	}
	return graphemeTarget(flat, off, dir)
}

func graphemeTarget(flat string, off int, dir vocab.Direction) (int, bool) {
	if dir > 0 {
		if off >= len(flat) {
			return 0, false
		}
		return vocab.NextGrapheme(flat, off), true
	}
	if off <= 0 {
		return 0, false
	}
	return vocab.PrevGrapheme(flat, off), true
}

// maskNodes keeps offsets aligned with the slot while making an inline
// node a word and sentence boundary. Its own start and end remain units.
func maskNodes(flat string) string {
	if !strings.ContainsRune(flat, doc.NodeByte) {
		return flat
	}
	b := []byte(flat)
	for i := range b {
		if b[i] == doc.NodeByte {
			b[i] = ' '
		}
	}
	return string(b)
}

// delete deletes what a selection covers, or one unit from a collapsed
// caret, through the same range operation every other deletion reads.
func delete(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Delete)
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Backward
	}
	sel := c.Selection()
	if sel.Kind == doc.SelectNode || sel.Kind == doc.SelectText && !sel.IsCollapsed() {
		from, to := c.Range()
		if from == to {
			return policy.Swallow
		}
		return policy.Do(vocab.DeleteRange(from, to))
	}
	if sel.Kind != doc.SelectText {
		return policy.Swallow
	}
	target, ok := targetAt(c, sel.Head, in.Unit, dir)
	if !ok || target.Pos == sel.Head.Pos {
		return policy.Swallow
	}
	return policy.Do(vocab.DeleteRange(sel.Head.Pos, target.Pos))
}

// place puts the caret where a pointer hit. A double click selects the
// segment under it; a triple click selects the slot's text.
func place(c *policy.Ctx, m policy.Match) policy.Result {
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
		anchor := sel.Anchor
		if in.Anchor != nil {
			anchor = *in.Anchor
		}
		return revealed(doc.TextRange(anchor, stop))
	}
	content, inline := c.Inline(stop.Pos.Ref())
	switch {
	case inline && in.Clicks >= 3:
		return revealed(doc.TextRange(
			doc.CaretAt(doc.At(stop.Pos.Ref(), 0)),
			doc.CaretAt(doc.At(stop.Pos.Ref(), content.Len())),
		))
	case inline && in.Clicks == 2:
		from, to := segmentAt(content, stop.Pos.Offset)
		return revealed(doc.TextRange(
			doc.CaretAt(doc.At(stop.Pos.Ref(), from)),
			doc.CaretAt(doc.At(stop.Pos.Ref(), to)),
		))
	}
	return policy.Select(doc.Collapsed(stop))
}

// segmentAt is the word segment a double click selects. At a boundary it
// prefers the segment ending there; at the slot's end, the last one.
func segmentAt(in *doc.Inline, off int) (int, int) {
	var units []span
	for segment := range vocab.WordSegments(maskNodes(in.Flat())) {
		units = append(units, span{segment.Start, segment.Start + len(segment.Text)})
	}
	in.Nodes(func(at int, _ *doc.Node) bool {
		units = append(units, span{at, at + 1})
		return true
	})
	sort.Slice(units, func(i, j int) bool {
		if units[i].from != units[j].from {
			return units[i].from < units[j].from
		}
		return units[i].to < units[j].to
	})
	if len(units) == 0 {
		return off, off
	}
	for _, u := range units {
		if u.from < off && off < u.to {
			return u.from, u.to
		}
	}
	for _, u := range units {
		if u.to == off {
			return u.from, u.to
		}
	}
	for _, u := range units {
		if u.from == off {
			return u.from, u.to
		}
	}
	if off >= in.Len() {
		last := units[len(units)-1]
		return last.from, last.to
	}
	return off, off
}

func revealed(s doc.Selection) policy.Result {
	return policy.Result{Selection: &s, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

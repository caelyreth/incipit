// Package words is the words policy: motion, selection and deletion by
// word, sentence and paragraph. Words use the ICU-tailored UAX #29 data in
// vocab; an inline node is one unit. Core keeps grapheme, line and page
// motion, and remains the fallback when this policy is absent.
package words

import (
	"sort"
	"strings"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "words"

// Policy is the words policy. It answers a caret's units; core owns what
// a range or a selected object does first, for every unit.
var Policy = policy.Define(ID, func(p *policy.P) {
	for _, unit := range []struct {
		name string
		unit vocab.Unit
	}{
		{"word", vocab.ByWord},
		{"sentence", vocab.BySentence},
		{"paragraph", vocab.ByParagraph},
	} {
		policy.On(p, "move-"+unit.name, policy.Moving(unit.unit, 0), policy.Slot.Inside(), move)
		policy.On(p, "extend-"+unit.name, policy.Extending(unit.unit, 0), policy.Slot.Inside(), move).Caret().Range()
		policy.On(p, "delete-"+unit.name, policy.Deleting(unit.unit, 0), policy.Slot.Inside(), delete)
	}
	policy.On(p, "select-word", policy.Clicking(vocab.DoubleClick), policy.Slot.Inside(), selectWord).Any()

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Move by whole words"),
		policy.Waits(400*time.Millisecond),
		policy.Presses(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Backward}),
		policy.Presses(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Backward}),
		policy.Waits(300*time.Millisecond),
		policy.Presses(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward, Extend: true}),
	)
}, policy.Label("policies.words"), policy.Description("policies.words.description"))

type span struct{ from, to int }

// move moves the head by the unit: the caret, or a range's head when it
// extends.
func move(c *policy.Ctx, in vocab.Move, _ policy.At[doc.SlotRef]) policy.Result {
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Forward
	}
	sel := c.Selection()
	target, ok := targetAt(c, sel.Head, in.Unit, dir)
	if !ok {
		return policy.Swallow
	}
	if in.Extend {
		return policy.Reveal(doc.TextRange(sel.Anchor, target))
	}
	return policy.Reveal(doc.Collapsed(target))
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
				return c.Stops().Normalize(caret), true
			}
			if head.Stop == 0 {
				return head, false
			}
		}
	}
	return c.Stops().Step(head, dir)
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
		return vocab.GraphemeStep(flat, off, dir)
	}
	for i := len(units) - 1; i >= 0; i-- {
		if units[i].from < off {
			return units[i].from, true
		}
	}
	return vocab.GraphemeStep(flat, off, dir)
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
	return vocab.GraphemeStep(flat, off, dir)
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
	return vocab.GraphemeStep(flat, off, dir)
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

// delete deletes one unit from the caret, through the same range
// operation every other deletion reads.
func delete(c *policy.Ctx, in vocab.Delete, _ policy.At[doc.SlotRef]) policy.Result {
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Backward
	}
	head := c.Head()
	target, ok := targetAt(c, head, in.Unit, dir)
	if !ok || target.Pos == head.Pos || target.Pos.Ref() != head.Pos.Ref() {
		// At its slot's edge: what lies across it is core's to delete, as
		// for any unit.
		return policy.Pass
	}
	return policy.Do(vocab.DeleteRange(head.Pos, target.Pos))
}

// selectWord selects the word segment a double click hits.
func selectWord(c *policy.Ctx, in vocab.Place, _ policy.At[doc.SlotRef]) policy.Result {
	stop := in.Stop
	content, ok := c.Inline(stop.Pos.Ref())
	if in.Node != 0 || !ok {
		return policy.Pass
	}
	from, to := segmentAt(content, stop.Pos.Offset, in.Over)
	return policy.Reveal(doc.TextRange(doc.CaretAt(doc.At(stop.Pos.Ref(), from)), doc.CaretAt(doc.At(stop.Pos.Ref(), to))))
}

// segmentAt is the word segment a double click selects: the one holding
// the grapheme the pointer is over, on the over side of off (before it
// when the side is not known); with none there, as at a slot's edge, the
// one on the other side.
func segmentAt(in *doc.Inline, off int, over vocab.Direction) (int, int) {
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
	if over == 0 {
		over = vocab.Backward
	}
	for _, dir := range []vocab.Direction{over, -over} {
		for _, u := range units {
			if dir == vocab.Forward && u.from <= off && off < u.to || dir == vocab.Backward && u.from < off && off <= u.to {
				return u.from, u.to
			}
		}
	}
	return off, off
}

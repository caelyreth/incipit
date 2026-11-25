// Package words is the words policy: motion, selection and deletion by
// word, sentence and paragraph. Words use the ICU-tailored UAX #29 data in
// vocab; an inline node is one unit. Core keeps grapheme, line and page
// motion, and remains the fallback when this policy is absent.
package words

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "words"

var NearbyContent = policy.Bool("nearbyContent", false, policy.Label("settings.words.nearbyContent"), policy.Description("settings.words.nearbyContent.description"))

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
	// Look Up asks the host to show what a word means: the selection's,
	// or the one a force click is on.
	p.Command(profile.LookUpCommand, "commands.lookUp", profile.LookUp{}, policy.Offer(profile.LookUpOffers, 0, ""))
	policy.On(p, "look-up", policy.Intent[profile.LookUp](), policy.Slot.Inside(), lookUp).Any()

	nearby := policy.Get(p, NearbyContent)
	policy.On(p, "select-word", policy.Clicking(vocab.DoubleClick), policy.Slot.Inside(), func(context *policy.Ctx, input vocab.Place, _ policy.At[doc.SlotRef]) policy.Result {
		return selectWord(context, input, nearby)
	}).Any()
	policy.On(p, "select-sentence", policy.Clicking(vocab.TripleClick), policy.Slot.Inside(), selectSentence).Any()

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

// lookUp asks the host to show what a word means: the one At is over,
// the selection left as it is; else the text selected in one slot, or
// the word at the caret. Formulas' source is not words, and nothing else
// is a word to look up.
func lookUp(c *policy.Ctx, in profile.LookUp, _ policy.At[doc.SlotRef]) policy.Result {
	var ref doc.SlotRef
	var from, to int
	switch sel := c.Selection(); {
	case in.At != nil:
		if in.At.Node != 0 {
			return policy.Pass
		}
		content, ok := c.Inline(in.At.Stop.Pos.Ref())
		if !ok {
			return policy.Pass
		}
		w, ok := wordOver(content, in.At.Stop.Pos.Offset, in.At.Over)
		if !ok {
			return policy.Pass
		}
		ref, from, to = in.At.Stop.Pos.Ref(), w.From, w.To
	case sel.Kind == doc.SelectText && sel.IsCollapsed():
		head := c.Head().Pos
		content, ok := c.Inline(head.Ref())
		if !ok {
			return policy.Pass
		}
		w, ok := vocab.WordAt(content, head.Offset)
		if !ok {
			return policy.Pass
		}
		ref, from, to = head.Ref(), w.From, w.To
	case sel.Kind == doc.SelectText:
		lo, hi := c.Range()
		if lo.Ref() != hi.Ref() {
			return policy.Pass
		}
		ref, from, to = lo.Ref(), lo.Offset, hi.Offset
	default:
		return policy.Pass
	}
	if n, ok := c.Doc().Node(ref.Node); !ok || n.Kind() == profile.Formula || n.Kind() == profile.Math {
		return policy.Pass
	}
	content, ok := c.Inline(ref)
	if !ok {
		return policy.Pass
	}
	text := strings.TrimSpace(content.Flat()[from:to])
	if text == "" {
		return policy.Pass
	}
	return policy.Result{Effects: []policy.Effect{{Kind: policy.EffectLookUp,
		Data: profile.LookUpAsk{Slot: ref, From: from, To: to, Text: text}}}}
}

// wordOver is the word holding the grapheme on the over side of off,
// before it where the side is not known; none over a space or a sign.
func wordOver(in *doc.Inline, off int, over vocab.Direction) (vocab.TextUnit, bool) {
	for _, w := range vocab.Words(in) {
		if w.Word && (over == vocab.Forward && w.From <= off && off < w.To || over != vocab.Forward && w.From < off && off <= w.To) {
			return w, true
		}
	}
	return vocab.TextUnit{}, false
}

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
		return sentenceTarget(in, flat, off, dir)
	case vocab.ByParagraph:
		return paragraphTarget(flat, off, dir)
	}
	return 0, false
}

// wordTarget passes spaces and punctuation to the next word-like segment.
// From inside a word it lands at that word's end; backward it lands at the
// current or previous word's start. Inline nodes are units of their own.
func wordTarget(in *doc.Inline, flat string, off int, dir vocab.Direction) (int, bool) {
	units := wordUnits(in)
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

// wordUnits are the slot's words, an inline node among them.
func wordUnits(in *doc.Inline) []span {
	var units []span
	for _, w := range vocab.Words(in) {
		if w.Word {
			units = append(units, span{w.From, w.To})
		}
	}
	return units
}

// sentenceTarget lands at a sentence's end going forward, and at the
// current or previous sentence's start going backward.
func sentenceTarget(in *doc.Inline, flat string, off int, dir vocab.Direction) (int, bool) {
	var units []span
	for _, s := range vocab.Sentences(in) {
		units = append(units, span{s.From, s.To})
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
func selectWord(c *policy.Ctx, in vocab.Place, nearby bool) policy.Result {
	stop := in.Stop
	content, ok := c.Inline(stop.Pos.Ref())
	if in.Node != 0 || !ok {
		return policy.Pass
	}
	from, to := segmentAt(content, stop.Pos.Offset, in.Over, nearby)
	return policy.Reveal(doc.TextRange(doc.CaretAt(doc.At(stop.Pos.Ref(), from)), doc.CaretAt(doc.At(stop.Pos.Ref(), to))))
}

// selectSentence selects the sentence a triple click hits, using the same
// sentence span as the selection ladder; with none there, core takes the slot.
func selectSentence(c *policy.Ctx, in vocab.Place, _ policy.At[doc.SlotRef]) policy.Result {
	stop := in.Stop
	content, ok := c.Inline(stop.Pos.Ref())
	if in.Node != 0 || in.Cells != nil || !ok {
		return policy.Pass
	}
	s, ok := vocab.SentenceAround(content, stop.Pos.Offset, stop.Pos.Offset)
	if !ok {
		return policy.Pass
	}
	return policy.Reveal(doc.TextRange(doc.CaretAt(doc.At(stop.Pos.Ref(), s.From)), doc.CaretAt(doc.At(stop.Pos.Ref(), s.To))))
}

// segmentAt is the word segment a double click selects: the one holding
// the grapheme the pointer is over, on the over side of off (before it
// when the side is not known); with none there, as at a slot's edge, the
// one on the other side.
func segmentAt(in *doc.Inline, off int, over vocab.Direction, nearby bool) (int, int) {
	flat := in.Flat()
	var units []span
	for _, w := range vocab.Words(in) {
		units = append(units, span{w.From, w.To})
	}
	if over == 0 {
		over = vocab.Backward
	}
	for _, dir := range []vocab.Direction{over, -over} {
		for index, unit := range units {
			if dir == vocab.Forward && unit.from <= off && off < unit.to || dir == vocab.Backward && unit.from < off && off <= unit.to {
				if nearby && horizontalSpace(flat[unit.from:unit.to]) {
					unit = nearestContent(flat, units, index, off, over)
				}
				return unit.from, unit.to
			}
		}
	}
	return off, off
}

func horizontalSpace(text string) bool {
	return text != "" && strings.IndexFunc(text, func(char rune) bool {
		return !unicode.IsSpace(char) || strings.ContainsRune("\r\n\v\f\u0085\u2028\u2029", char)
	}) < 0
}

func nearestContent(flat string, units []span, index, off int, over vocab.Direction) span {
	left, right := index-1, index+1
	for left >= 0 && horizontalSpace(flat[units[left].from:units[left].to]) {
		left--
	}
	for right < len(units) && horizontalSpace(flat[units[right].from:units[right].to]) {
		right++
	}
	before := left >= 0 && strings.TrimSpace(flat[units[left].from:units[left].to]) != ""
	after := right < len(units) && strings.TrimSpace(flat[units[right].from:units[right].to]) != ""
	if before && after {
		backward := utf8.RuneCountInString(flat[units[left].to:off])
		forward := utf8.RuneCountInString(flat[off:units[right].from])
		if backward < forward || backward == forward && over == vocab.Backward {
			return units[left]
		}
		return units[right]
	}
	if before {
		return units[left]
	}
	if after {
		return units[right]
	}
	return units[index]
}

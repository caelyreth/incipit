package scopes

import (
	"slices"
	"strings"
	"unicode"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The selection ladder: ExpandSelection climbs the rungs around the
// selection, the word at the caret, then the scopes Select All grows
// through with the sentence among them, and ShrinkSelection comes back
// down the rungs climbed. Select All climbs the scopes alone, from the
// block, and the two share the record of what was climbed, so either
// comes back down.

// rung is one step climbed: from the selection it started at to the one
// it selected, at, over the steps climbed before it. A rung is the
// policy's mode while the selection is still at; once anything else
// selects, it no longer stands, and shrinking has nothing to go back to.
type rung struct {
	at, from doc.Selection
	below    *rung
}

// climbed is the rung that selected sel, if the view still stands on it.
func climbed(c *policy.Ctx, sel doc.Selection) *rung {
	if r, ok := c.Mode(ID).(*rung); ok && sameSelection(c.Doc(), r.at, sel) {
		return r
	}
	return nil
}

// expand climbs one rung of the whole ladder.
func expand(c *policy.Ctx, _ vocab.ExpandSelection, _ policy.At[doc.SlotRef]) policy.Result {
	cands := append(candidatesAt(c), textRungs(c)...)
	order(c.Doc(), cands)
	return climb(c, dedupe(c.Doc(), cands))
}

// shrink goes back to where the last rung climbed from, the rung below
// standing again; with nothing climbed to the selection, it does nothing.
func shrink(c *policy.Ctx, _ vocab.ShrinkSelection, _ policy.At[doc.SlotRef]) policy.Result {
	r := climbed(c, c.Selection())
	if r == nil {
		return policy.Swallow
	}
	res := policy.Select(r.from)
	if r.below != nil {
		res.Mode = r.below
	} else {
		res.DropMode = true
	}
	return res
}

// textRungs are the rungs of text the scopes do not hold, within the one
// text slot the selection stands in: the word at it, and its sentence.
// A word is had from a caret in or at the edge of one; a sentence holds
// the selection, trailing spaces left out.
func textRungs(c *policy.Ctx) []candidate {
	sel := c.Selection()
	if sel.Kind != doc.SelectText || sel.Anchor.Pos.Ref() != sel.Head.Pos.Ref() {
		return nil
	}
	ref := sel.Head.Pos.Ref()
	content, ok := c.Inline(ref)
	if !ok {
		return nil
	}
	flat := content.Flat()
	from, to := sel.Anchor.Pos.Offset, sel.Head.Pos.Offset
	if from > to {
		from, to = to, from
	}
	span := func(f, t int) candidate {
		return candidate{selection: settled(c, doc.TextRange(doc.CaretAt(doc.At(ref, f)), doc.CaretAt(doc.At(ref, t))))}
	}
	var out []candidate
	if from == to {
		if f, t, ok := wordAt(flat, from); ok {
			out = append(out, span(f, t))
		}
	}
	for s := range vocab.SentenceSegments(flat) {
		f, t := s.Start, s.Start+len(strings.TrimRightFunc(s.Text, unicode.IsSpace))
		if f <= from && to <= t && f < t {
			out = append(out, span(f, t))
			break
		}
	}
	return out
}

// wordAt is the word holding off, or ending or starting at it, the one
// after it first where two meet there: only a word, never the space or
// the punctuation between words.
func wordAt(flat string, off int) (int, int, bool) {
	var before [2]int
	found := false
	for w := range vocab.WordSegments(flat) {
		f, t := w.Start, w.Start+len(w.Text)
		if !w.WordLike {
			continue
		}
		if f <= off && off < t {
			return f, t, true
		}
		if t == off {
			before, found = [2]int{f, t}, true
		}
	}
	return before[0], before[1], found
}

// dedupe drops the rungs of an extent another before it has: a sentence
// that is its block's whole text is one rung.
func dedupe(r doc.Reader, cands []candidate) []candidate {
	out := cands[:0:0]
	for _, cand := range cands {
		if !slices.ContainsFunc(out, func(o candidate) bool { return sameSelection(r, o.selection, cand.selection) }) {
			out = append(out, cand)
		}
	}
	return out
}

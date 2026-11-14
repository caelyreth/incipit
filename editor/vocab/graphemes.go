package vocab

import (
	"unicode/utf8"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Grapheme boundaries are UAX #29 extended grapheme clusters. Word,
// sentence and paragraph boundaries belong to the words policy; grapheme
// boundaries remain core's.

// NextGrapheme returns the offset of the grapheme boundary after off in s.
func NextGrapheme(s string, off int) int {
	if off <= 0 {
		if s == "" {
			return 0
		}
		iter := graphemes.FromString(s)
		iter.Next()
		return iter.End()
	}
	if off >= len(s) {
		return len(s)
	}
	b := boundaryAtOrBefore(s, off)
	iter := graphemes.FromString(s[b:])
	for iter.Next() {
		if b+iter.End() > off {
			return b + iter.End()
		}
	}
	return len(s)
}

// PrevGrapheme returns the offset of the grapheme boundary before off.
func PrevGrapheme(s string, off int) int {
	if off <= 0 {
		return 0
	}
	if off >= len(s) {
		off = len(s)
	}
	b := boundaryAtOrBefore(s, off-1)
	iter := graphemes.FromString(s[b:])
	prev := b
	for iter.Next() {
		if b+iter.End() >= off {
			return b + iter.Start()
		}
		prev = b + iter.Start()
	}
	return prev
}

// boundaryAtOrBefore is a grapheme boundary at or before at, near it, from
// which segmenting finds the boundaries segmenting all of s finds. Between
// two ASCII characters but CR LF there is always one (UAX #29: neither is
// Extend, SpacingMark, ZWJ, Prepend or a regional indicator), so the search
// goes back to the nearest such pair, or to the start.
func boundaryAtOrBefore(s string, at int) int {
	if at >= len(s) {
		return len(s)
	}
	for i := at; i > 0; i-- {
		if s[i-1] < utf8.RuneSelf && s[i] < utf8.RuneSelf && (s[i-1] != '\r' || s[i] != '\n') {
			return i
		}
	}
	return 0
}

// GraphemeCount is the number of graphemes in s.
func GraphemeCount(s string) int {
	n := 0
	iter := graphemes.FromString(s)
	for iter.Next() {
		n++
	}
	return n
}

// GraphemeAt returns the grapheme containing off and its byte bounds. At a
// boundary it is the grapheme starting there. It is false outside s.
func GraphemeAt(s string, off int) (text string, start, end int, ok bool) {
	if off < 0 || off >= len(s) {
		return "", 0, 0, false
	}
	b := boundaryAtOrBefore(s, off)
	iter := graphemes.FromString(s[b:])
	for iter.Next() {
		if b+iter.End() > off {
			return iter.Value(), b + iter.Start(), b + iter.End(), true
		}
	}
	return "", 0, 0, false
}

// ClassOfGrapheme returns the classes of a grapheme cluster by its base
// rune, so an atom classifies the whole cluster rather than whichever rune
// happens to touch the caret.
func ClassOfGrapheme(g string) CharClass {
	r, _ := utf8.DecodeRuneInString(g)
	return ClassOf(r)
}

// IsLineBreakGrapheme reports a grapheme that separates lines.
func IsLineBreakGrapheme(g string) bool {
	switch g {
	case "\n", "\r", "\r\n":
		return true
	}
	return false
}

// GraphemeClassAt returns the class of the grapheme before or after off.
// Its report is false for an inline node, which is not text and is classed
// by no character class.
func GraphemeClassAt(s string, off int, before bool) (CharClass, bool) {
	if before {
		if off <= 0 {
			return 0, true
		}
		start := PrevGrapheme(s, off)
		end := NextGrapheme(s, start)
		if start < 0 || start >= len(s) || end <= start {
			return 0, true
		}
		g := s[start:end]
		if g[0] == doc.NodeByte {
			return 0, false
		}
		if IsLineBreakGrapheme(g) {
			return 0, true
		}
		return ClassOfGrapheme(g), true
	}
	if off >= len(s) {
		return 0, true
	}
	text, _, _, ok := GraphemeAt(s, off)
	if !ok {
		return 0, true
	}
	if text[0] == doc.NodeByte {
		return 0, false
	}
	if IsLineBreakGrapheme(text) {
		return 0, true
	}
	return ClassOfGrapheme(text), true
}

// GraphemeStep is the grapheme boundary one step from off in dir; false at
// the text's edge that way.
func GraphemeStep(s string, off int, dir Direction) (int, bool) {
	if dir > 0 {
		if off >= len(s) {
			return off, false
		}
		return NextGrapheme(s, off), true
	}
	if off <= 0 {
		return off, false
	}
	return PrevGrapheme(s, off), true
}

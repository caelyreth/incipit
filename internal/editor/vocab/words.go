package vocab

import (
	"iter"
	"strings"
	"unicode"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/text/wordseg"
	"github.com/clipperhouse/uax29/v2/sentences"
)

// WordSegment is one word-granularity segment: its text, byte start and
// whether it is word-like rather than space or punctuation.
type WordSegment = wordseg.Segment

// WordSegments returns the word segments of text through the ICU-tailored
// word rules used by the rest of the product.
func WordSegments(text string) iter.Seq[WordSegment] {
	return wordseg.Segments(text)
}

// CountWords is how many words text holds: its word-like segments. The
// page's counts and the notebook's summaries both count by it.
func CountWords(text string) int {
	n := 0
	for s := range WordSegments(text) {
		if s.WordLike {
			n++
		}
	}
	return n
}

// SentenceSegment is one UAX #29 sentence segment: its text and byte
// start.
type SentenceSegment struct {
	Text  string
	Start int
}

// SentenceSegments returns the sentence segments of text.
func SentenceSegments(text string) iter.Seq[SentenceSegment] {
	return func(yield func(SentenceSegment) bool) {
		iter := sentences.FromString(text)
		for iter.Next() {
			if !yield(SentenceSegment{Text: iter.Value(), Start: iter.Start()}) {
				return
			}
		}
	}
}

// TextUnit is one unit of an inline slot's text, [From, To) in its
// bytes: a word segment, or a sentence. Word is set on a word-like one,
// as against the space or punctuation between words.
type TextUnit struct {
	From, To int
	Word     bool
}

// Words are an inline slot's word segments, in order, every byte in one:
// an inline node is a word of its own, and bounds the words around it.
// Every reader of words in a slot (moving and deleting by word, a double
// click, the selection's ladder) reads these, so they agree on what a
// word is.
func Words(in *doc.Inline) []TextUnit {
	flat := in.Flat()
	var out []TextUnit
	for s := range WordSegments(maskNodes(flat)) {
		from, to := s.Start, s.Start+len(s.Text)
		// A masked node reads as a space: split it out as its own word.
		at := from
		for i := from; i < to; i++ {
			if flat[i] != doc.NodeByte {
				continue
			}
			if at < i {
				out = append(out, TextUnit{From: at, To: i})
			}
			out = append(out, TextUnit{From: i, To: i + 1, Word: true})
			at = i + 1
		}
		if at < to {
			out = append(out, TextUnit{From: at, To: to, Word: s.WordLike && at == from})
		}
	}
	return out
}

// Sentences are an inline slot's sentences, in order; an inline node
// bounds none.
func Sentences(in *doc.Inline) []TextUnit {
	var out []TextUnit
	for s := range SentenceSegments(maskNodes(in.Flat())) {
		out = append(out, TextUnit{From: s.Start, To: s.Start + len(s.Text)})
	}
	return out
}

// WordAt is the word holding off, or ending or starting at it, the one
// after it first where two meet there: only a word, never the space or
// the punctuation between words.
func WordAt(in *doc.Inline, off int) (TextUnit, bool) {
	var before TextUnit
	found := false
	for _, w := range Words(in) {
		if !w.Word {
			continue
		}
		if w.From <= off && off < w.To {
			return w, true
		}
		if w.To == off {
			before, found = w, true
		}
	}
	return before, found
}

// SentenceAround is the sentence holding [from, to), its trailing spaces
// left out.
func SentenceAround(in *doc.Inline, from, to int) (TextUnit, bool) {
	flat := in.Flat()
	for _, s := range Sentences(in) {
		end := s.From + len(strings.TrimRightFunc(flat[s.From:s.To], unicode.IsSpace))
		if s.From <= from && to <= end && s.From < end {
			return TextUnit{From: s.From, To: end}, true
		}
	}
	return TextUnit{}, false
}

// maskNodes keeps offsets aligned with the slot while making an inline
// node a word and sentence boundary.
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

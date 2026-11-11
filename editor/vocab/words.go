package vocab

import (
	"iter"

	"github.com/caelyreth/neverland/internal/wordseg"
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

package vocab

import (
	"iter"

	"github.com/caelyreth/neverland/internal/wordseg"
)

// WordSegment is one word-granularity segment: its text, byte start and
// whether it is word-like rather than space or punctuation.
type WordSegment = wordseg.Segment

// WordSegments returns the word segments of text through the ICU-tailored
// word rules used by the rest of the product.
func WordSegments(text string) iter.Seq[WordSegment] {
	return wordseg.Segments(text)
}

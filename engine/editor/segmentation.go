package editor

import (
	"slices"
	"unicode"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/clipperhouse/uax29/v2/sentences"
	"github.com/clipperhouse/uax29/v2/words"
)

// Segment is one half-open byte range produced by a UAX #29 boundary walk.
type Segment struct {
	Range model.ByteRange
}

// WordSegment extends a UAX #29 segment with the word-like classification
// used by editor statistics and word movement.
type WordSegment struct {
	Range    model.ByteRange
	WordLike bool
}

// UTF16Segment is a segment translated into scalar-checked UTF-16 coordinates.
type UTF16Segment struct {
	Range    model.UTF16Range
	WordLike bool
}

// Words returns all UAX #29 word segments, including whitespace and
// punctuation. Thai runs use the ICU4X codepoint LSTM, as Rust Shoreward does.
// The text must be valid UTF-8.
func Words(text string) ([]WordSegment, bool) {
	if !model.ValidDocumentText(text) {
		return nil, false
	}
	iterator := words.FromString(text)
	uax29 := make([]WordSegment, 0)
	for iterator.Next() {
		uax29 = append(uax29, WordSegment{
			Range: model.NewByteRange(model.ByteOffset(iterator.Start()), model.ByteOffset(iterator.End())),
		})
	}
	segments := make([]WordSegment, 0, len(uax29))
	for index := 0; index < len(uax29); {
		start := uax29[index].Range.Start
		value := text[start:uax29[index].Range.End]
		if !thaiSegment(value) {
			segments = append(segments, WordSegment{
				Range:    uax29[index].Range,
				WordLike: wordLike(value),
			})
			index++
			continue
		}

		end := uax29[index].Range.End
		next := index + 1
		for next < len(uax29) &&
			uax29[next].Range.Start == end &&
			thaiSegment(text[uax29[next].Range.Start:uax29[next].Range.End]) {
			end = uax29[next].Range.End
			next++
		}
		for _, boundary := range loadThaiLSTM().segment(text[start:end]) {
			segments = append(segments, WordSegment{
				Range: model.NewByteRange(
					start+model.ByteOffset(boundary.start),
					start+model.ByteOffset(boundary.end),
				),
				WordLike: true,
			})
		}
		index = next
	}
	return segments, true
}

// WordLikeRanges returns only UAX #29 word-like ranges. This mirrors ICU4X
// WordType.is_word_like(): letters and numbers qualify, spaces, punctuation,
// symbols, and emoji do not.
func WordLikeRanges(text string) ([]model.ByteRange, bool) {
	segments, ok := Words(text)
	if !ok {
		return nil, false
	}
	ranges := make([]model.ByteRange, 0, len(segments))
	for _, segment := range segments {
		if segment.WordLike {
			ranges = append(ranges, segment.Range)
		}
	}
	return ranges, true
}

// SentenceRanges returns the UAX #29 sentence segments. Trailing whitespace is
// part of the preceding sentence, matching ICU4X and uax29.
func SentenceRanges(text string) ([]model.ByteRange, bool) {
	if !model.ValidDocumentText(text) {
		return nil, false
	}
	iterator := sentences.FromString(text)
	ranges := make([]model.ByteRange, 0)
	for iterator.Next() {
		ranges = append(ranges, model.NewByteRange(
			model.ByteOffset(iterator.Start()),
			model.ByteOffset(iterator.End()),
		))
	}
	return ranges, true
}

// GraphemeRanges returns UAX #29 extended grapheme clusters. These are for
// editor movement and deletion policy, never semantic anchor validation.
func GraphemeRanges(text string) ([]model.ByteRange, bool) {
	if !model.ValidDocumentText(text) {
		return nil, false
	}
	iterator := graphemes.FromString(text)
	ranges := make([]model.ByteRange, 0)
	for iterator.Next() {
		ranges = append(ranges, model.NewByteRange(
			model.ByteOffset(iterator.Start()),
			model.ByteOffset(iterator.End()),
		))
	}
	return ranges, true
}

// UTF16Graphemes converts grapheme byte ranges into checked UTF-16 ranges.
func UTF16Graphemes(text string) ([]model.UTF16Range, bool) {
	ranges, ok := GraphemeRanges(text)
	if !ok {
		return nil, false
	}
	converted := make([]model.UTF16Range, 0, len(ranges))
	for _, byteRange := range ranges {
		rangeUTF16, ok := model.ByteRangeToUTF16(text, byteRange)
		if !ok {
			return nil, false
		}
		converted = append(converted, rangeUTF16)
	}
	return converted, true
}

// WordRangesUTF16 converts all word segments into UTF-16 coordinates.
func WordRangesUTF16(text string) ([]UTF16Segment, bool) {
	segments, ok := Words(text)
	if !ok {
		return nil, false
	}
	converted := make([]UTF16Segment, 0, len(segments))
	for _, segment := range segments {
		rangeUTF16, ok := model.ByteRangeToUTF16(text, segment.Range)
		if !ok {
			return nil, false
		}
		converted = append(converted, UTF16Segment{Range: rangeUTF16, WordLike: segment.WordLike})
	}
	return converted, true
}

// PreviousGrapheme returns the cluster immediately before a scalar-aligned
// UTF-16 offset. A caret inside a cluster walks to the cluster start.
func PreviousGrapheme(text string, offset model.UTF16Offset) (model.UTF16Range, bool) {
	ranges, ok := UTF16Graphemes(text)
	if !ok || offset < 0 || int(offset) > model.StringLenUTF16(text) {
		return model.UTF16Range{}, false
	}
	index := slices.IndexFunc(ranges, func(current model.UTF16Range) bool {
		return current.Start < offset && offset <= current.End
	})
	if index < 0 {
		return model.UTF16Range{}, false
	}
	return ranges[index], true
}

// NextGrapheme returns the cluster containing or immediately after a
// scalar-aligned UTF-16 offset. A caret inside a cluster therefore expands to
// that cluster before moving.
func NextGrapheme(text string, offset model.UTF16Offset) (model.UTF16Range, bool) {
	ranges, ok := UTF16Graphemes(text)
	if !ok || offset < 0 || int(offset) >= model.StringLenUTF16(text) {
		return model.UTF16Range{}, false
	}
	index := slices.IndexFunc(ranges, func(current model.UTF16Range) bool {
		return current.Start <= offset && offset < current.End
	})
	if index < 0 {
		return model.UTF16Range{}, false
	}
	return ranges[index], true
}

// GraphemeAt expands a scalar offset to its containing cluster. At a cluster
// boundary it returns that cluster; at end of text it returns an empty range.
func GraphemeAt(text string, offset model.UTF16Offset) (model.UTF16Range, bool) {
	ranges, ok := UTF16Graphemes(text)
	if !ok || offset < 0 || int(offset) > model.StringLenUTF16(text) {
		return model.UTF16Range{}, false
	}
	if int(offset) == model.StringLenUTF16(text) {
		return model.EmptyUTF16Range(offset), true
	}
	index := slices.IndexFunc(ranges, func(current model.UTF16Range) bool {
		return current.Start <= offset && offset < current.End
	})
	if index < 0 {
		return model.UTF16Range{}, false
	}
	return ranges[index], true
}

func wordLike(value string) bool {
	hasBase := false
	hasExtendNumLet := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			hasBase = true
		case r == '_':
			hasExtendNumLet = true
		}
	}
	return hasBase || hasExtendNumLet
}

func thaiSegment(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '\u0e01' || '\u0e4e' < r {
			return false
		}
	}
	return true
}

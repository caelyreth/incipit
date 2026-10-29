package editor

import (
	"slices"

	"github.com/caelyreth/neverland/engine/editor/internal/wordsegmenter"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/clipperhouse/uax29/v2/sentences"
)

// Segment is one half-open byte range produced by a UAX #29 boundary walk.
type Segment struct {
	Range model.ByteRange
}

// WordSegment extends a word boundary segment with the ICU word-like
// classification used by editor statistics and word movement.
type WordSegment struct {
	Range    model.ByteRange
	WordLike bool
}

// UTF16Segment is a segment translated into scalar-checked UTF-16 coordinates.
type UTF16Segment struct {
	Range    model.UTF16Range
	WordLike bool
}

// Words returns all ICU-tailored UAX #29 word segments, including whitespace
// and punctuation. Chinese and Japanese use the ICU dictionary engine. Deferred
// language-specific scripts use the Unicode fallback. The text must be valid
// UTF-8 without NUL.
func Words(text string) ([]WordSegment, bool) {
	scanned, ok := scanWordSegments(text)
	if !ok {
		return nil, false
	}
	segments := make([]WordSegment, 0, len(scanned))
	for _, segment := range scanned {
		segments = append(segments, WordSegment{
			Range:    segment.byteRange,
			WordLike: segment.wordLike,
		})
	}
	return segments, true
}

// WordLikeRanges returns only word-like ranges. Letters and numbers qualify;
// spaces, punctuation, symbols, and emoji do not.
func WordLikeRanges(text string) ([]model.ByteRange, bool) {
	segments, ok := scanWordSegments(text)
	if !ok {
		return nil, false
	}
	ranges := make([]model.ByteRange, 0, len(segments))
	for _, segment := range segments {
		if segment.wordLike {
			ranges = append(ranges, segment.byteRange)
		}
	}
	return ranges, true
}

// SentenceRanges returns the UAX #29 sentence segments. Trailing whitespace is
// part of the preceding sentence.
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
	segments, ok := scanWordSegments(text)
	if !ok {
		return nil, false
	}
	converted := make([]UTF16Segment, 0, len(segments))
	for _, segment := range segments {
		converted = append(converted, UTF16Segment{
			Range:    segment.utf16Range,
			WordLike: segment.wordLike,
		})
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

type scannedWordSegment struct {
	byteRange  model.ByteRange
	utf16Range model.UTF16Range
	wordLike   bool
}

func scanWordSegments(text string) ([]scannedWordSegment, bool) {
	if !model.ValidDocumentText(text) {
		return nil, false
	}
	segments := make([]scannedWordSegment, 0)
	byteOffset, utf16Offset := 0, 0
	for segment := range wordsegmenter.Segments(text) {
		end := byteOffset + len(segment.Segment)
		if segment.Segment == "" ||
			segment.Index != utf16Offset ||
			end > len(text) ||
			text[byteOffset:end] != segment.Segment {
			return nil, false
		}
		utf16End := utf16Offset + model.StringLenUTF16(segment.Segment)
		segments = append(segments, scannedWordSegment{
			byteRange: model.NewByteRange(
				model.ByteOffset(byteOffset),
				model.ByteOffset(end),
			),
			utf16Range: model.NewUTF16Range(
				model.UTF16Offset(utf16Offset),
				model.UTF16Offset(utf16End),
			),
			wordLike: segment.IsWordLike,
		})
		byteOffset = end
		utf16Offset = utf16End
	}
	if byteOffset != len(text) || utf16Offset != model.StringLenUTF16(text) {
		return nil, false
	}
	return segments, true
}

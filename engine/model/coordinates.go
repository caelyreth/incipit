package model

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ByteOffset is an offset in UTF-8 bytes.
type ByteOffset int

// RuneOffset is an offset in Unicode scalar values.
type RuneOffset int

// UTF16Offset is an offset in UTF-16 code units.
type UTF16Offset int

// ByteRange is a half-open UTF-8 byte range. Construction does not validate
// text boundaries.
type ByteRange struct {
	Start ByteOffset
	End   ByteOffset
}

// RuneRange is a half-open Unicode scalar range.
type RuneRange struct {
	Start RuneOffset
	End   RuneOffset
}

// UTF16Range is a half-open UTF-16 range. Construction does not validate
// scalar boundaries.
type UTF16Range struct {
	Start UTF16Offset
	End   UTF16Offset
}

func NewByteRange(start, end ByteOffset) ByteRange {
	return ByteRange{Start: start, End: end}
}

func NewRuneRange(start, end RuneOffset) RuneRange {
	return RuneRange{Start: start, End: end}
}

func NewUTF16Range(start, end UTF16Offset) UTF16Range {
	return UTF16Range{Start: start, End: end}
}

func EmptyUTF16Range(at UTF16Offset) UTF16Range {
	return UTF16Range{Start: at, End: at}
}

func (r ByteRange) Empty() bool {
	return r.Start >= r.End
}

func (r ByteRange) Len() int {
	return max(0, int(r.End-r.Start))
}

func (r RuneRange) Empty() bool {
	return r.Start >= r.End
}

func (r RuneRange) Len() int {
	return max(0, int(r.End-r.Start))
}

func (r UTF16Range) Empty() bool {
	return r.Start >= r.End
}

func (r UTF16Range) Len() int {
	return max(0, int(r.End-r.Start))
}

// StringLenUTF16 returns the text's length in UTF-16 code units.
func StringLenUTF16(text string) int {
	units := 0
	for _, r := range text {
		units += utf16.RuneLen(r)
	}
	return units
}

// StringLenRunes returns the text's length in Unicode scalar values.
func StringLenRunes(text string) int {
	return utf8.RuneCountInString(text)
}

// RuneUTF16Len returns the number of UTF-16 code units a scalar occupies.
func RuneUTF16Len(r rune) int {
	return utf16.RuneLen(r)
}

// ByteOffsetToUTF16 converts a checked UTF-8 boundary to UTF-16. Invalid UTF-8,
// continuation bytes, and out-of-range bytes reject.
func ByteOffsetToUTF16(text string, offset ByteOffset) (UTF16Offset, bool) {
	i := int(offset)
	if i < 0 || i > len(text) || !utf8.ValidString(text) {
		return 0, false
	}
	if i == len(text) {
		return UTF16Offset(StringLenUTF16(text)), true
	}
	if !utf8.RuneStart(text[i]) {
		return 0, false
	}
	prefix := text[:i]
	if !utf8.ValidString(prefix) {
		return 0, false
	}
	return UTF16Offset(StringLenUTF16(prefix)), true
}

// RuneOffsetToByte converts a scalar offset to a checked byte boundary.
func RuneOffsetToByte(text string, offset RuneOffset) (ByteOffset, bool) {
	index := int(offset)
	if index < 0 || !utf8.ValidString(text) {
		return 0, false
	}
	if index == 0 {
		return 0, true
	}
	seen := 0
	for byteIndex := range text {
		if seen == index {
			return ByteOffset(byteIndex), true
		}
		seen++
	}
	if seen == index {
		return ByteOffset(len(text)), true
	}
	return 0, false
}

// UTF16OffsetToByte rejects offsets beyond the text and offsets inside a
// surrogate pair.
func UTF16OffsetToByte(text string, offset UTF16Offset) (ByteOffset, bool) {
	target := int(offset)
	if target < 0 || !utf8.ValidString(text) {
		return 0, false
	}
	units := 0
	for byteIndex, r := range text {
		if units == target {
			return ByteOffset(byteIndex), true
		}
		units += utf16.RuneLen(r)
		if units > target {
			return 0, false
		}
	}
	if units == target {
		return ByteOffset(len(text)), true
	}
	return 0, false
}

// UTF16OffsetToByteClamped caps beyond-end offsets and rounds forward when an
// offset falls inside a surrogate pair.
func UTF16OffsetToByteClamped(text string, offset UTF16Offset) ByteOffset {
	target := max(0, int(offset))
	if target == 0 {
		return 0
	}
	units := 0
	for byteIndex, r := range text {
		if units >= target {
			return ByteOffset(byteIndex)
		}
		units += utf16.RuneLen(r)
	}
	return ByteOffset(len(text))
}

// UTF16OffsetIsBoundary reports whether an offset is scalar-aligned and in
// range.
func UTF16OffsetIsBoundary(text string, offset UTF16Offset) bool {
	_, ok := UTF16OffsetToByte(text, offset)
	return ok
}

// ByteRangeToUTF16 validates a byte range and converts it to UTF-16.
func ByteRangeToUTF16(text string, r ByteRange) (UTF16Range, bool) {
	if r.Start > r.End {
		return UTF16Range{}, false
	}
	start, ok := ByteOffsetToUTF16(text, r.Start)
	if !ok {
		return UTF16Range{}, false
	}
	end, ok := ByteOffsetToUTF16(text, r.End)
	if !ok {
		return UTF16Range{}, false
	}
	return UTF16Range{Start: start, End: end}, true
}

// UTF16RangeToByte validates a UTF-16 range and converts it to UTF-8 bytes.
func UTF16RangeToByte(text string, r UTF16Range) (ByteRange, bool) {
	if r.Start > r.End {
		return ByteRange{}, false
	}
	start, ok := UTF16OffsetToByte(text, r.Start)
	if !ok {
		return ByteRange{}, false
	}
	end, ok := UTF16OffsetToByte(text, r.End)
	if !ok {
		return ByteRange{}, false
	}
	return ByteRange{Start: start, End: end}, true
}

// AlignUTF16RangeForNativeQuery scalar-aligns a native substring query by
// expanding split-surrogate endpoints outward. The end is capped at the text
// length; reversed or beyond-end starts reject.
func AlignUTF16RangeForNativeQuery(text string, r UTF16Range) (UTF16Range, ByteRange, bool) {
	if r.Start > r.End || !utf8.ValidString(text) {
		return UTF16Range{}, ByteRange{}, false
	}
	length := UTF16Offset(StringLenUTF16(text))
	if r.Start > length {
		return UTF16Range{}, ByteRange{}, false
	}
	start := r.Start
	end := min(r.End, length)
	if !UTF16OffsetIsBoundary(text, start) {
		start = max(0, start-1)
	}
	if !UTF16OffsetIsBoundary(text, end) {
		end++
	}
	aligned := UTF16Range{Start: start, End: end}
	bytes, ok := UTF16RangeToByte(text, aligned)
	if !ok {
		return UTF16Range{}, ByteRange{}, false
	}
	return aligned, bytes, true
}

// ValidDocumentText rejects invalid UTF-8 before it enters semantic storage.
func ValidDocumentText(text string) bool {
	return utf8.ValidString(text) && !strings.ContainsRune(text, '\x00')
}

// NormalizeLineEndings makes LF the one semantic line separator.
func NormalizeLineEndings(text string) string {
	if !strings.ContainsRune(text, '\r') {
		return text
	}
	var builder strings.Builder
	builder.Grow(len(text))
	for index := 0; index < len(text); index++ {
		if text[index] != '\r' {
			builder.WriteByte(text[index])
			continue
		}
		builder.WriteByte('\n')
		if index+1 < len(text) && text[index+1] == '\n' {
			index++
		}
	}
	return builder.String()
}

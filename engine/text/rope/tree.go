package rope

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Tree is an immutable version of a text. Its zero value is the empty text.
// Trees are safe to read concurrently and operations return new versions.
type Tree struct {
	root *node
}

// New returns an immutable tree containing text.
func New(text string) Tree {
	return Tree{root: build(text)}
}

// Same reports whether two values reference the same immutable text version.
func (t Tree) Same(other Tree) bool {
	return t.root == other.root
}

func (t Tree) Summary() Summary {
	return summaryOf(t.root)
}

func (t Tree) ByteLen() int {
	return t.Summary().Bytes
}

func (t Tree) RuneLen() int {
	return t.Summary().Runes
}

func (t Tree) UTF16Len() int {
	return t.Summary().Units
}

func (t Tree) LineCount() int {
	return t.Summary().Lines + 1
}

func (t Tree) IsASCII() bool {
	return t.ByteLen() == t.UTF16Len()
}

// Slice returns bytes start to end, clamped to the text.
func (t Tree) Slice(start, end int) string {
	start = max(0, min(start, t.ByteLen()))
	end = max(0, min(end, t.ByteLen()))
	if start >= end {
		return ""
	}
	var output strings.Builder
	output.Grow(end - start)
	read(t.root, start, end, &output)
	return output.String()
}

// String materializes the complete text.
func (t Tree) String() string {
	return t.Slice(0, t.ByteLen())
}

// Chunks invokes fn with each retained piece of bytes start to end until fn
// returns false.
func (t Tree) Chunks(start, end int, fn func(string) bool) {
	start = max(0, min(start, t.ByteLen()))
	end = max(0, min(end, t.ByteLen()))
	if start < end {
		_ = chunks(t.root, start, end, fn)
	}
}

// Replace returns a new version replacing bytes start to end. Reversed or
// out-of-range endpoints clamp to the text.
func (t Tree) Replace(start, end int, replacement string) Tree {
	return Tree{root: replaceBytes(t.root, start, end, replacement)}
}

func (t Tree) ByteOfRune(offset int) int {
	return byteOfRune(t.root, offset)
}

func (t Tree) RuneOfByte(offset int) int {
	return prefix(t.root, max(0, min(offset, t.ByteLen()))).Runes
}

func (t Tree) RuneOfUTF16(offset int) int {
	return runeOfUnit(t.root, max(0, min(offset, t.UTF16Len())))
}

func (t Tree) ByteOfUTF16(offset int) int {
	return byteOfUnitRoundedDown(t.root, max(0, min(offset, t.UTF16Len())))
}

// CheckedByteOfUTF16 rejects surrogate interiors and beyond-end offsets.
func (t Tree) CheckedByteOfUTF16(offset int) (int, bool) {
	return byteOfUnitExact(t.root, offset)
}

// CheckedUTF16OfByte rejects continuation and out-of-range bytes.
func (t Tree) CheckedUTF16OfByte(offset int) (int, bool) {
	if offset < 0 || offset > t.ByteLen() {
		return 0, false
	}
	if offset == t.ByteLen() {
		return t.UTF16Len(), true
	}
	r, size := t.runeAt(offset)
	if r == utf8.RuneError && size == 1 {
		return 0, false
	}
	return prefix(t.root, offset).Units, true
}

func (t Tree) runeAt(offset int) (rune, int) {
	if offset < 0 || offset >= t.ByteLen() {
		return 0, 0
	}
	window := min(utf8.UTFMax, t.ByteLen()-offset)
	return utf8.DecodeRuneInString(t.Slice(offset, offset+window))
}

// RuneAt returns the rune starting at offset.
func (t Tree) RuneAt(offset int) (rune, int) {
	return t.runeAt(offset)
}

// RuneBefore returns the rune ending at offset.
func (t Tree) RuneBefore(offset int) (rune, int) {
	if offset <= 0 || offset > t.ByteLen() {
		return 0, 0
	}
	start := max(0, offset-utf8.UTFMax)
	return utf8.DecodeLastRuneInString(t.Slice(start, offset))
}

// SliceUTF16 returns a validated UTF-16 slice and actual byte range.
func (t Tree) SliceUTF16(start, end int) (string, int, int, bool) {
	if start > end {
		return "", 0, 0, false
	}
	startByte, ok := t.CheckedByteOfUTF16(start)
	if !ok {
		return "", 0, 0, false
	}
	endByte, ok := t.CheckedByteOfUTF16(end)
	if !ok {
		return "", 0, 0, false
	}
	return t.Slice(startByte, endByte), startByte, endByte, true
}

// LineStart returns the byte at which a zero-based line starts, clamped.
func (t Tree) LineStart(line int) int {
	line = max(0, min(line, t.LineCount()-1))
	return lineStart(t.root, line)
}

// LineEnd returns the byte before a line's newline.
func (t Tree) LineEnd(line int) int {
	line = max(0, min(line, t.LineCount()-1))
	if line+1 < t.LineCount() {
		return t.LineStart(line+1) - 1
	}
	return t.ByteLen()
}

// LineOf returns the zero-based line containing a byte offset.
func (t Tree) LineOf(offset int) int {
	return prefix(t.root, max(0, min(offset, t.ByteLen()))).Lines
}

// Line returns a line without its newline.
func (t Tree) Line(line int) string {
	return t.Slice(t.LineStart(line), t.LineEnd(line))
}

// EachLine streams lines [from,to) without materializing the document.
func (t Tree) EachLine(from, to int, fn func(index, start int, line string) bool) {
	to = min(to, t.LineCount())
	for index := max(0, from); index < to; index++ {
		if !fn(index, t.LineStart(index), t.Line(index)) {
			return
		}
	}
}

// UTF16UnitsIn returns units in bytes start to end.
func (t Tree) UTF16UnitsIn(start, end int) int {
	start = max(0, min(start, t.ByteLen()))
	end = max(0, min(end, t.ByteLen()))
	if start >= end {
		return 0
	}
	return prefix(t.root, end).Units - prefix(t.root, start).Units
}

// RuneLenIn returns scalar count in bytes start to end.
func (t Tree) RuneLenIn(start, end int) int {
	start = max(0, min(start, t.ByteLen()))
	end = max(0, min(end, t.ByteLen()))
	if start >= end {
		return 0
	}
	return prefix(t.root, end).Runes - prefix(t.root, start).Runes
}

// UTF16LenOfRune is exported for tests of summary metrics without exposing the
// rope node.
func UTF16LenOfRune(r rune) int {
	return utf16.RuneLen(r)
}

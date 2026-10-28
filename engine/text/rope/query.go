package rope

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func decodeRune(text string) (rune, int) {
	return utf8.DecodeRuneInString(text)
}

func byteOfRune(n *node, index int) int {
	index = max(0, min(index, summaryOf(n).Runes))
	offset := 0
	for n != nil && n.left != nil {
		if index < n.left.summary.Runes {
			n = n.left
		} else {
			index -= n.left.summary.Runes
			offset += n.left.summary.Bytes
			n = n.right
		}
	}
	if n == nil {
		return offset
	}
	if n.summary.Bytes == n.summary.Runes {
		return offset + index
	}
	for at := range n.text {
		if index == 0 {
			return offset + at
		}
		index--
	}
	return offset + len(n.text)
}

func runeOfUnit(n *node, index int) int {
	offset := 0
	for n != nil && n.left != nil {
		if index < n.left.summary.Units {
			n = n.left
		} else {
			index -= n.left.summary.Units
			offset += n.left.summary.Runes
			n = n.right
		}
	}
	if n == nil {
		return offset
	}
	if n.summary.Runes == n.summary.Units {
		return offset + min(index, n.summary.Runes)
	}
	for _, r := range n.text {
		units := utf16.RuneLen(r)
		if index < units {
			return offset
		}
		index -= units
		offset++
	}
	return offset
}

func byteOfUnitRoundedDown(n *node, index int) int {
	offset := 0
	for n != nil && n.left != nil {
		if index < n.left.summary.Units {
			n = n.left
		} else {
			index -= n.left.summary.Units
			offset += n.left.summary.Bytes
			n = n.right
		}
	}
	if n == nil {
		return offset
	}
	for at, r := range n.text {
		units := utf16.RuneLen(r)
		if index < units {
			return offset + at
		}
		index -= units
	}
	return offset + len(n.text)
}

// byteOfUnitExact returns a byte offset only if unit lies on a scalar
// boundary. Otherwise it reports the containing rune's start and false.
func byteOfUnitExact(n *node, unit int) (int, bool) {
	if unit < 0 || unit > summaryOf(n).Units {
		return 0, false
	}
	offset := 0
	for n != nil && n.left != nil {
		if unit < n.left.summary.Units {
			n = n.left
		} else {
			unit -= n.left.summary.Units
			offset += n.left.summary.Bytes
			n = n.right
		}
	}
	if n == nil {
		if unit == 0 {
			return offset, true
		}
		return offset, false
	}
	if unit == 0 {
		return offset, true
	}
	for at, r := range n.text {
		units := utf16.RuneLen(r)
		if unit == units {
			return offset + at + len(string(r)), true
		}
		if unit < units {
			return offset + at, false
		}
		unit -= units
	}
	return offset + len(n.text), unit == 0
}

func prefix(n *node, byteOffset int) Summary {
	if n == nil || byteOffset <= 0 {
		return Summary{}
	}
	if byteOffset >= n.summary.Bytes {
		return n.summary
	}
	if n.left == nil {
		return summarize(n.text[:byteOffset])
	}
	if byteOffset <= n.left.summary.Bytes {
		return prefix(n.left, byteOffset)
	}
	return n.left.summary.add(prefix(n.right, byteOffset-n.left.summary.Bytes))
}

func lineStart(n *node, line int) int {
	if line <= 0 {
		return 0
	}
	if n == nil {
		return 0
	}
	if line > n.summary.Lines {
		return n.summary.Bytes
	}
	if n.left == nil {
		at := 0
		for count := 0; count < line; count++ {
			next := strings.IndexByte(n.text[at:], '\n')
			if next < 0 {
				return n.summary.Bytes
			}
			at += next + 1
		}
		return at
	}
	if line <= n.left.summary.Lines {
		return lineStart(n.left, line)
	}
	return n.left.summary.Bytes + lineStart(n.right, line-n.left.summary.Lines)
}

func read(n *node, start, end int, builder *strings.Builder) {
	if n == nil || start >= end {
		return
	}
	if start <= 0 && end >= n.summary.Bytes {
		if n.left == nil {
			builder.WriteString(n.text)
			return
		}
		read(n.left, 0, n.left.summary.Bytes, builder)
		read(n.right, 0, n.right.summary.Bytes, builder)
		return
	}
	if n.left == nil {
		builder.WriteString(n.text[max(0, start):min(end, len(n.text))])
		return
	}
	leftBytes := n.left.summary.Bytes
	if start < leftBytes {
		read(n.left, start, min(end, leftBytes), builder)
	}
	if end > leftBytes {
		read(n.right, max(0, start-leftBytes), end-leftBytes, builder)
	}
}

func chunks(n *node, start, end int, fn func(string) bool) bool {
	if n == nil || start >= end {
		return true
	}
	if start <= 0 && end >= n.summary.Bytes && n.left == nil {
		return fn(n.text)
	}
	if n.left == nil {
		return fn(n.text[max(0, start):min(end, len(n.text))])
	}
	leftBytes := n.left.summary.Bytes
	if start < leftBytes && !chunks(n.left, start, min(end, leftBytes), fn) {
		return false
	}
	if end > leftBytes {
		return chunks(n.right, max(0, start-leftBytes), end-leftBytes, fn)
	}
	return true
}

package vocab

import (
	"unicode"
	"unicode/utf8"
)

// Grapheme boundaries, approximating UAX #29 extended grapheme clusters:
// a base followed by extenders (combining marks, variation selectors,
// emoji modifiers, ZWJ sequences), CR LF, and regional indicator pairs.
//
// TODO(words): replace with the full UAX #29 tables the words policy
// brings (ICU-tailored), keeping these signatures.

func isExtender(r rune) bool {
	switch {
	case r == 0x200D: // ZWJ
		return true
	case r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF: // variation selectors
		return true
	case r >= 0x1F3FB && r <= 0x1F3FF: // emoji modifiers
		return true
	case r >= 0xE0020 && r <= 0xE007F: // tags
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc)
}

func isRegional(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

// NextGrapheme returns the offset of the grapheme boundary after off in s.
func NextGrapheme(s string, off int) int {
	if off >= len(s) {
		return len(s)
	}
	r, n := utf8.DecodeRuneInString(s[off:])
	i := off + n
	if r == '\r' && i < len(s) && s[i] == '\n' {
		return i + 1
	}
	if r == '\n' || r == '\r' {
		return i
	}
	if isRegional(r) && i < len(s) {
		if r2, n2 := utf8.DecodeRuneInString(s[i:]); isRegional(r2) {
			i += n2
		}
	}
	for i < len(s) {
		r2, n2 := utf8.DecodeRuneInString(s[i:])
		if !isExtender(r2) {
			break
		}
		i += n2
		// A ZWJ joins the next pictograph into the cluster.
		if r2 == 0x200D && i < len(s) {
			_, n3 := utf8.DecodeRuneInString(s[i:])
			i += n3
		}
	}
	return i
}

// PrevGrapheme returns the offset of the grapheme boundary before off.
func PrevGrapheme(s string, off int) int {
	if off <= 0 {
		return 0
	}
	// Walk forward from a safe start: the line start, or a bounded
	// distance back, since clusters are short.
	start := max(0, off-64)
	for start > 0 && !utf8.RuneStart(s[start]) {
		start--
	}
	prev := start
	for i := start; i < off; {
		prev = i
		i = NextGrapheme(s, i)
		if i > off {
			return prev
		}
	}
	return prev
}

// GraphemeCount is the number of graphemes in s.
func GraphemeCount(s string) int {
	n := 0
	for off := 0; off < len(s); off = NextGrapheme(s, off) {
		n++
	}
	return n
}

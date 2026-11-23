package vocab

import (
	"unicode"
)

// CharClass is a set of character classes. A line's edge, a line break
// and an inline node have none.
type CharClass uint16

const (
	ClassWord CharClass = 1 << iota
	ClassSpace
	ClassCJK
	ClassDigit
)

// IsCJK reports a Han, kana, Hangul or CJK punctuation character.
func IsCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo) ||
		(r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFFEF)
}

// ClassOf returns the classes of r.
func ClassOf(r rune) CharClass {
	var c CharClass
	switch {
	case r == '\n':
		return 0
	case unicode.IsSpace(r):
		c |= ClassSpace
	case unicode.IsDigit(r):
		c |= ClassDigit | ClassWord
	case unicode.IsLetter(r):
		c |= ClassWord
	}
	if IsCJK(r) {
		c |= ClassCJK
	}
	return c
}

// FirstClass is the class of text's first grapheme, or none for empty
// text.
func FirstClass(text string) CharClass {
	g, _, _, ok := GraphemeAt(text, 0)
	if !ok {
		return 0
	}
	return ClassOfGrapheme(g)
}

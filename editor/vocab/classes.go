package vocab

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CharClass is a set of character classes, as PrevChar and NextChar test
// them.
type CharClass uint16

const (
	ClassWord CharClass = 1 << iota
	ClassSpace
	ClassCJK
	ClassLatin
	ClassDigit
	ClassPunct
	ClassOpener
	ClassCloser
	// ClassLineStart and ClassLineEnd stand for no neighbour: the caret is
	// at the start or end of a line.
	ClassLineStart
	ClassLineEnd
)

const openers = "([{“‘«「『（【《〈〔［｛"
const closers = ")]}”’»」』）】》〉〕］｝"

// Closer returns the closer matching an opener.
func Closer(open rune) (rune, bool) {
	i := strings.IndexRune(openers, open)
	if i < 0 {
		return 0, false
	}
	n := utf8.RuneCountInString(openers[:i])
	for j, r := range []rune(closers) {
		if j == n {
			return r, true
		}
	}
	return 0, false
}

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
		return ClassLineEnd
	case unicode.IsSpace(r):
		c |= ClassSpace
	case unicode.IsDigit(r):
		c |= ClassDigit | ClassWord
	case unicode.IsLetter(r):
		c |= ClassWord
		if unicode.Is(unicode.Latin, r) {
			c |= ClassLatin
		}
	case unicode.IsPunct(r) || unicode.IsSymbol(r):
		c |= ClassPunct
	}
	if IsCJK(r) {
		c |= ClassCJK
	}
	if strings.ContainsRune(openers, r) {
		c |= ClassOpener
	}
	if strings.ContainsRune(closers, r) {
		c |= ClassCloser
	}
	return c
}

// FirstRune is the first rune of s and its length.
func FirstRune(s string) (rune, int) {
	if s == "" {
		return 0, 0
	}
	return utf8.DecodeRuneInString(s)
}

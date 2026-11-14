package query

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/wordseg"
	"golang.org/x/text/unicode/norm"
)

// Token is one term a text yields: its folded form, the byte range of the
// text it was read from, and its position. Tokens at one position stand for
// the same place in the text, as a character and the pair it starts.
type Token struct {
	Term       string
	Start, End int
	Pos        int
}

// Mode is what a text is read for.
type Mode uint8

const (
	// Indexed reads every term a later query can look for: words, and for
	// text written without spaces (Chinese, Japanese, Korean) each
	// character, each pair of characters, and the dictionary's words.
	Indexed Mode = iota
	// Queried reads a query's words as the terms that find them: words,
	// and for text written without spaces the pairs it holds, or its one
	// character, so a run of characters finds itself inside a longer one.
	Queried
)

// Tokens reads text into terms.
//
// Words are wordseg's, folded (Fold). Adjacent word-like segments written
// without spaces form one run, whose characters take a position each;
// Indexed gives each character, the pair it starts and the dictionary word
// it starts there, Queried gives the pairs (the lone character of a run of
// one), leaving the position of a run's last character empty, so a phrase
// that goes on past it lines up with the index.
func Tokens(text string, mode Mode) []Token {
	var out []Token
	pos := 0
	var run []wordseg.Segment
	flush := func() {
		if len(run) > 0 {
			out, pos = appendRun(out, text, run, pos, mode)
			run = run[:0]
		}
	}
	for seg := range wordseg.Segments(text) {
		if !seg.WordLike {
			flush()
			continue
		}
		if r, _ := utf8.DecodeRuneInString(seg.Text); Unspaced(r) {
			if len(run) > 0 {
				last := run[len(run)-1]
				if last.Start+len(last.Text) != seg.Start {
					flush()
				}
			}
			run = append(run, seg)
			continue
		}
		flush()
		pos++
		out = append(out, Token{Term: Fold(seg.Text), Start: seg.Start, End: seg.Start + len(seg.Text), Pos: pos})
	}
	flush()
	return out
}

// appendRun adds a run of segments written without spaces, its first
// character at the position after pos, and returns the position of its
// last character.
func appendRun(out []Token, text string, run []wordseg.Segment, pos int, mode Mode) ([]Token, int) {
	from := run[0].Start
	last := run[len(run)-1]
	to := last.Start + len(last.Text)
	// starts[i] is the byte where the run's ith character starts.
	var starts []int
	for i := range text[from:to] {
		starts = append(starts, from+i)
	}
	n := len(starts)
	starts = append(starts, to)
	at := func(i int) int { return pos + 1 + i }
	char := func(i, k int) string { return Fold(text[starts[i]:starts[i+k]]) }
	switch {
	case mode == Queried && n == 1:
		out = append(out, Token{Term: char(0, 1), Start: starts[0], End: starts[1], Pos: at(0)})
	case mode == Queried:
		for i := 0; i+1 < n; i++ {
			out = append(out, Token{Term: char(i, 2), Start: starts[i], End: starts[i+2], Pos: at(i)})
		}
	default:
		words := map[int]int{}
		for _, seg := range run {
			k := utf8.RuneCountInString(seg.Text)
			if k > 2 {
				words[index(starts, seg.Start)] = k
			}
		}
		for i := range n {
			out = append(out, Token{Term: char(i, 1), Start: starts[i], End: starts[i+1], Pos: at(i)})
			if i+1 < n {
				out = append(out, Token{Term: char(i, 2), Start: starts[i], End: starts[i+2], Pos: at(i)})
			}
			if k, ok := words[i]; ok {
				out = append(out, Token{Term: char(i, k), Start: starts[i], End: starts[i+k], Pos: at(i)})
			}
		}
	}
	return out, at(n - 1)
}

// index is the character a byte offset starts in starts.
func index(starts []int, b int) int {
	for i, s := range starts {
		if s >= b {
			return i
		}
	}
	return len(starts) - 1
}

// Unspaced reports whether r is written without spaces between words.
func Unspaced(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// Fold is the form a term is compared in: compatibility forms joined
// (full width as normal width), lower case, marks taken from letters
// (café as cafe) but kept on kana, whose voicing marks spell other sounds,
// and katakana as hiragana.
func Fold(s string) string {
	if ascii(s) {
		return strings.ToLower(s)
	}
	s = strings.ToLower(norm.NFKC.String(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r) && r != 0x3099 && r != 0x309A:
			continue
		case r >= 0x30A1 && r <= 0x30F6:
			r -= 0x60
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}

func ascii(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

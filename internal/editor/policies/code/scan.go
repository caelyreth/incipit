package code

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
)

// scanner reads code a byte at a time. Whatever spans bytes, and so may
// span the tokens the framework cuts the text in, is its state: the piece
// open and where it began, the word so far, and the bytes just read, by
// which a delimiter is known as its last byte arrives. What it reads is
// thus the same however the text is cut.
type scanner struct {
	kinds [classComment + 1]*policy.FactKind[token]
}

// mode is the piece of code the scanner is in.
type mode uint8

const (
	modeCode mode = iota
	modeWord
	modeNumber
	modeString
	// modeEmpty is a string closed as soon as it opened, which a third
	// quote makes a triple-quoted one.
	modeEmpty
	modeTriple
	modeLine
	modeBlock
)

// wordRoom is how many bytes of a word the state keeps: a longer word is
// none the language names.
const wordRoom = 16

// state is the scanner's state between bytes.
type state struct {
	lang  string
	mode  mode
	start int
	// quote is the open string's quote, block the open comment's index.
	quote byte
	block uint8
	esc   bool
	// word is the word so far, n its length.
	word [wordRoom]byte
	n    uint8
	// back are the last bytes read, the latest last.
	back [3]byte
}

func (s scanner) Start(u *policy.Unit) state {
	m, _ := u.Context.(profile.Marker)
	return state{lang: profile.CodeLanguage(m.Info)}
}

func (s scanner) Step(st state, t policy.Token, emit func(policy.Fact)) state {
	lx := lexiconOf(st.lang)
	if lx == nil {
		return st
	}
	switch t.Kind {
	case policy.TokenText:
		for i := 0; i < len(t.Text); i++ {
			st = s.read(lx, st, t.Text[i], t.At+i, emit)
		}
	case policy.TokenEnd:
		st = s.end(lx, st, t.At, emit)
	}
	return st
}

// read reads byte c at offset at.
func (s scanner) read(lx *lexicon, st state, c byte, at int, emit func(policy.Fact)) state {
	switch st.mode {
	case modeWord:
		if wordByte(lx, c) {
			if int(st.n) < wordRoom {
				st.word[st.n] = c
			}
			st.n = min(st.n+1, wordRoom+1)
			return push(st, c)
		}
		st = s.end(lx, st, at, emit)
	case modeNumber:
		if isDigit(c) || isLetter(c) || c == '_' || c == '.' {
			return push(st, c)
		}
		st = s.end(lx, st, at, emit)
	case modeString, modeTriple:
		switch {
		case st.esc:
			st.esc = false
		case c == '\\' && strings.IndexByte(lx.raw, st.quote) < 0:
			st.esc = true
		case c == st.quote && st.mode == modeString:
			if lx.triple && at == st.start+1 {
				st.mode = modeEmpty
				return push(st, c)
			}
			emit(s.kinds[classString].Span(st.start, at+1, token{}))
			return reset(st, c)
		case c == st.quote && st.back[1] == c && st.back[2] == c && at-2 > st.start+2:
			emit(s.kinds[classString].Span(st.start, at+1, token{}))
			return reset(st, c)
		case c == '\n' && st.mode == modeString && strings.IndexByte(lx.multi, st.quote) < 0:
			// A string its line ends is closed there.
			emit(s.kinds[classString].Span(st.start, at, token{}))
			return reset(st, c)
		}
		return push(st, c)
	case modeEmpty:
		if c == st.quote {
			st.mode = modeTriple
			return push(st, c)
		}
		st = s.end(lx, st, at, emit)
	case modeLine:
		if c == '\n' {
			st = s.end(lx, st, at, emit)
			return push(st, c)
		}
		return push(st, c)
	case modeBlock:
		close := lx.blocks[st.block][1]
		if at-st.start >= len(lx.blocks[st.block][0])+len(close)-1 && ends(st, c, close) {
			emit(s.kinds[classComment].Span(st.start, at+1, token{}))
			return reset(st, c)
		}
		return push(st, c)
	}
	// In code: a comment is known by its opener's last byte, a string by
	// its quote, a number by its first digit and a word by its first
	// letter.
	for _, open := range lx.line {
		if ends(st, c, open) {
			st.mode, st.start = modeLine, at+1-len(open)
			return push(st, c)
		}
	}
	for i, b := range lx.blocks {
		if ends(st, c, b[0]) {
			st.mode, st.start, st.block = modeBlock, at+1-len(b[0]), uint8(i)
			return push(st, c)
		}
	}
	switch {
	case strings.IndexByte(lx.quotes, c) >= 0:
		st.mode, st.start, st.quote, st.esc = modeString, at, c, false
	case isDigit(c):
		st.mode, st.start = modeNumber, at
	case wordByte(lx, c) && !isDigit(c):
		st.mode, st.start, st.n = modeWord, at, 1
		st.word[0] = c
	}
	return push(st, c)
}

// end closes the piece open at offset at, emitting it, and goes back to
// code.
func (s scanner) end(lx *lexicon, st state, at int, emit func(policy.Fact)) state {
	switch st.mode {
	case modeWord:
		if int(st.n) <= wordRoom {
			w := string(st.word[:st.n])
			if lx.fold {
				w = strings.ToLower(w)
			}
			if c, ok := lx.words[w]; ok {
				emit(s.kinds[c].Span(st.start, at, token{}))
			}
		}
	case modeNumber:
		emit(s.kinds[classNumber].Span(st.start, at, token{}))
	case modeString, modeEmpty, modeTriple:
		emit(s.kinds[classString].Span(st.start, at, token{}))
	case modeLine, modeBlock:
		emit(s.kinds[classComment].Span(st.start, at, token{}))
	}
	st.mode, st.n, st.esc = modeCode, 0, false
	return st
}

// push records c as the last byte read.
func push(st state, c byte) state {
	st.back = [3]byte{st.back[1], st.back[2], c}
	return st
}

// reset goes back to code after c, which closed a piece: no byte of the
// piece begins a delimiter after it.
func reset(st state, _ byte) state {
	st.mode, st.n, st.esc = modeCode, 0, false
	st.back = [3]byte{}
	return st
}

// ends reports whether the bytes read, then c, end with delim.
func ends(st state, c byte, delim string) bool {
	n := len(delim)
	if n == 0 || n > len(st.back)+1 || delim[n-1] != c {
		return false
	}
	for i := 0; i < n-1; i++ {
		if st.back[len(st.back)-(n-1)+i] != delim[i] {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// wordByte reports a byte a word holds: letters, digits, '_', the
// language's own, and every byte of a rune past ASCII.
func wordByte(lx *lexicon, c byte) bool {
	return isLetter(c) || isDigit(c) || c == '_' || c >= utf8.RuneSelf || strings.IndexByte(lx.word, c) >= 0
}

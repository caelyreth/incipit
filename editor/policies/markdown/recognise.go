package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// run is an unclosed delimiter run: an opener whose closer may be typed
// anywhere later on its line. Its fact's inner extent runs from after the
// opener to the line's end, so the caret inside it is in its scope.
type run struct {
	Char byte
	N    int
}

// half is a single delimiter closing half of a two-character run, as
// "**bold*" waits for its second star.
type half struct{ Char byte }

// marker is a block prefix at a paragraph's start that a typed space turns
// into structure.
type marker struct {
	Kind  markerKind
	Level int // heading level, or the ordered list's start
	Task  profile.Task
	Char  byte
}

type markerKind uint8

const (
	markHeading markerKind = iota + 1
	markQuote
	markBullet
	markOrdered
	markTaskOpen
	markTaskDone
)

// fence is a whole paragraph that Enter turns into a block: a code fence
// with its info string, a divider, or a math fence.
type fence struct {
	Kind fenceKind
	Info string
}

type fenceKind uint8

const (
	fenceCode fenceKind = iota + 1
	fenceDivider
	fenceMath
)

// inlineConfig is which delimiter runs the analyzer recognises.
type inlineConfig struct {
	Asterisks, Underscores, Code, Strike bool
}

// runKinds are the fact kinds of runs, one per spelling, so rules are
// expanded per spelling when the policy is built.
type runKinds struct {
	em, strong, halves map[byte]*policy.FactKind[run]
	halfKinds          map[byte]*policy.FactKind[half]
	code, strike       *policy.FactKind[run]
}

func (k *runKinds) all() []policy.FactID {
	var out []policy.FactID
	for _, c := range []byte{'*', '_'} {
		out = append(out, k.em[c].Kind(), k.strong[c].Kind(), k.halfKinds[c].Kind())
	}
	return append(out, k.code.Kind(), k.strike.Kind())
}

type openRun struct {
	char  byte
	n     int
	start int
	half  int // offset of a half closer, or -1
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' }

// analyzeRuns finds the unclosed delimiter runs of one line. Code spans
// hide other delimiters; closed pairs are not facts (they were text the
// user kept, such as pasted Markdown, which the importer handles).
func analyzeRuns(cfg inlineConfig, k *runKinds) func(u *policy.Unit, emit func(policy.Fact)) {
	return func(u *policy.Unit, emit func(policy.Fact)) {
		if u.Spec.Literal {
			return
		}
		t := strings.TrimSuffix(u.Text(), "\n")
		base := u.From
		end := base + len(t)
		var stack []openRun
		code := -1
		for i := 0; i < len(t); {
			c := t[i]
			if c == '`' && cfg.Code {
				// Only a lone backtick delimits code; longer strings are
				// fences or literal.
				if n := len(t[i:]) - len(strings.TrimLeft(t[i:], "`")); n > 1 {
					i += n
					continue
				}
				if code >= 0 {
					code = -1
				} else {
					code = i
				}
				i++
				continue
			}
			if code >= 0 || !(c == '*' && cfg.Asterisks || c == '_' && cfg.Underscores || c == '~' && cfg.Strike) {
				i++
				continue
			}
			n := 1
			for i+n < len(t) && t[i+n] == c {
				n++
			}
			prevSpace := i == 0 || isSpace(t[i-1])
			nextSpace := i+n >= len(t) || isSpace(t[i+n])
			closed := false
			if !prevSpace {
				for j := len(stack) - 1; j >= 0; j-- {
					o := stack[j]
					if o.char != c {
						continue
					}
					if o.n == 2 && n == 1 && c != '~' {
						// A half closer: the second delimiter is still to
						// come.
						stack[j].half = i
					} else {
						stack = stack[:j]
					}
					closed = true
					break
				}
			}
			prev, _ := utf8.DecodeLastRuneInString(t[:i])
			prevWord := i > 0 && vocab.ClassOf(prev)&vocab.ClassWord != 0
			intraword := c == '_' && prevWord
			// An opener at the line's end, after no word, waits for what
			// is typed next: content opens the mark (§12), a space keeps
			// the delimiter literal.
			waits := i+n >= len(t) && !prevWord
			if !closed && (!nextSpace || waits) && !intraword && n <= 2 && !(c == '~' && n != 1) {
				stack = append(stack, openRun{char: c, n: n, start: i, half: -1})
			}
			i += n
		}
		if code >= 0 {
			emit(k.code.Delimited(base+code, base+code+1, end, end, run{Char: '`', N: 1}))
		}
		for _, o := range stack {
			data := run{Char: o.char, N: o.n}
			from, inner := base+o.start, base+o.start+o.n
			switch {
			case o.char == '~':
				emit(k.strike.Delimited(from, inner, end, end, data))
			case o.n == 1:
				emit(k.em[o.char].Delimited(from, inner, end, end, data))
			default:
				emit(k.strong[o.char].Delimited(from, inner, end, end, data))
				if o.half >= 0 {
					emit(k.halfKinds[o.char].Span(base+o.half, base+o.half+1, half{Char: o.char}))
				}
			}
		}
	}
}

// blockConfig is which block prefixes the analyzer recognises.
type blockConfig struct {
	Headings, Lists, Quotes, Code, Math, Rules bool
}

// analyzeBlocks finds a paragraph's block marker and fence.
func analyzeBlocks(cfg blockConfig, mk *policy.FactKind[marker], fk *policy.FactKind[fence]) func(u *policy.Unit, emit func(policy.Fact)) {
	return func(u *policy.Unit, emit func(policy.Fact)) {
		t := u.Text()
		if t == "" {
			return
		}
		if u.Owner.Kind() == profile.Item {
			if !cfg.Lists {
				return
			}
			switch {
			case strings.HasPrefix(t, string(profile.TaskOpenSpelling)):
				emit(mk.Span(0, len(profile.TaskOpenSpelling), marker{Kind: markTaskOpen, Task: profile.TaskOpen}))
			case strings.HasPrefix(t, string(profile.TaskDoneSpelling)),
				strings.HasPrefix(t, "[X]"):
				emit(mk.Span(0, len(profile.TaskDoneSpelling), marker{Kind: markTaskDone, Task: profile.TaskDone}))
			}
			return
		}
		switch {
		case cfg.Headings && t[0] == '#':
			n := 0
			for n < len(t) && t[n] == '#' {
				n++
			}
			if n <= 6 {
				emit(mk.Span(0, n, marker{Kind: markHeading, Level: n}))
			}
		case cfg.Quotes && t[0] == '>':
			emit(mk.Span(0, 1, marker{Kind: markQuote}))
		case cfg.Lists && (t[0] == '-' || t[0] == '*' || t[0] == '+'):
			emit(mk.Span(0, 1, marker{Kind: markBullet, Char: t[0]}))
		case cfg.Lists && t[0] >= '0' && t[0] <= '9':
			n := 0
			for n < len(t) && n < 9 && t[n] >= '0' && t[n] <= '9' {
				n++
			}
			if n < len(t) && (t[n] == '.' || t[n] == ')') {
				start, _ := profile.ParseOrdered(t[:n] + ".")
				emit(mk.Span(0, n+1, marker{Kind: markOrdered, Level: start, Char: t[n]}))
			}
		}
		switch {
		case cfg.Code && strings.HasPrefix(t, string(profile.FenceSpelling)) && !strings.Contains(t[3:], "`"):
			emit(fk.Span(0, len(t), fence{Kind: fenceCode, Info: strings.TrimSpace(t[3:])}))
		case cfg.Math && t == string(profile.MathSpelling):
			emit(fk.Span(0, len(t), fence{Kind: fenceMath}))
		case cfg.Rules && len(t) >= 3 && strings.Count(t, t[:1]) == len(t) && (t[0] == '-' || t[0] == '*' || t[0] == '_'):
			emit(fk.Span(0, len(t), fence{Kind: fenceDivider}))
		}
	}
}

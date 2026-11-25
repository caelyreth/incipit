package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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
// with its info string, a divider, a math fence, or a table's row, its
// cells' texts in Info.
type fence struct {
	Kind fenceKind
	Info string
}

type fenceKind uint8

const (
	fenceCode fenceKind = iota + 1
	fenceDivider
	fenceMath
	fenceTable
)

// inlineConfig is which delimiter runs the analyzer recognises.
type inlineConfig struct {
	Asterisks, Underscores, Code, Strike bool
}

// runKinds are the fact kinds of runs. Emphasis, strong and their half
// closers are keyed by delimiter, so a rule stands in one spelling's runs.
type runKinds struct {
	em, strong   policy.KeyedKind[run, byte]
	half         policy.KeyedKind[half, byte]
	open         policy.KeyedKind[run, doc.MarkKind]
	code, strike *policy.FactKind[run]
}

func (k *runKinds) all() []policy.FactID {
	return []policy.FactID{k.em.Kind(), k.strong.Kind(), k.half.Kind(), k.open.Kind(), k.code.Kind(), k.strike.Kind()}
}

func markOfRun(r run) doc.MarkKind {
	switch {
	case r.Char == '`':
		return profile.CodeMark
	case r.Char == '~':
		return profile.Strike
	case r.N == 2:
		return profile.Strong
	default:
		return profile.Emphasis
	}
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
func analyzeRuns(cfg inlineConfig, k *runKinds) func(u *policy.Unit, e *policy.Emit) {
	return func(u *policy.Unit, e *policy.Emit) {
		if u.Spec.Literal {
			return
		}
		t := strings.TrimSuffix(u.Text(), "\n")
		base := u.From
		end := base + len(t)
		var stack []openRun
		var waits []openRun
		code := -1
		codeWait := false
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
					codeWait = false
				} else {
					code = i
					prev, _ := utf8.DecodeLastRuneInString(t[:i])
					prevWord := i > 0 && vocab.ClassOf(prev)&vocab.ClassWord != 0
					next, _ := utf8.DecodeRuneInString(t[i+1:])
					nextWord := i+1 < len(t) && vocab.ClassOf(next)&vocab.ClassWord != 0
					codeWait = !prevWord && !nextWord
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
					for w := len(waits) - 1; w >= 0; w-- {
						if waits[w].start >= o.start {
							waits = append(waits[:w], waits[w+1:]...)
						}
					}
					closed = true
					break
				}
			}
			prev, _ := utf8.DecodeLastRuneInString(t[:i])
			prevWord := i > 0 && vocab.ClassOf(prev)&vocab.ClassWord != 0
			next, _ := utf8.DecodeRuneInString(t[i+n:])
			nextWord := i+n < len(t) && vocab.ClassOf(next)&vocab.ClassWord != 0
			intraword := c == '_' && prevWord
			valid := !closed && !intraword && n <= 2 && !(c == '~' && n != 1)
			if valid && !nextSpace {
				stack = append(stack, openRun{char: c, n: n, start: i, half: -1})
			}
			// An unclosed opener not hugging adjacent words waits wherever it
			// stands on the line: typing content right after it opens the mark
			// (§12), while spaces and non-hugging text keep it literal.
			if valid && !prevWord && !nextWord {
				waits = append(waits, openRun{char: c, n: n, start: i, half: -1})
			}
			i += n
		}
		if code >= 0 {
			e.Fact(k.code.Delimited(base+code, base+code+1, end, end, run{Char: '`', N: 1}))
			if codeWait {
				e.Fact(k.open.Span(base+code, base+code+1, run{Char: '`', N: 1}))
			}
		}
		for _, o := range waits {
			data := run{Char: o.char, N: o.n}
			e.Fact(k.open.Span(base+o.start, base+o.start+o.n, data))
		}
		for _, o := range stack {
			data := run{Char: o.char, N: o.n}
			from, inner := base+o.start, base+o.start+o.n
			switch {
			case o.char == '~':
				e.Fact(k.strike.Delimited(from, inner, end, end, data))
			case o.n == 1:
				e.Fact(k.em.Delimited(from, inner, end, end, data))
			default:
				e.Fact(k.strong.Delimited(from, inner, end, end, data))
				if o.half >= 0 {
					e.Fact(k.half.Span(base+o.half, base+o.half+1, half{Char: o.char}))
				}
			}
		}
	}
}

// rule is a line of one rule character, typed so far: Enter makes a
// divider of three or more.
type rule struct{}

// bindRule is the rule's line, as Syntax.
func bindRule(at policy.At[rule]) vocab.Syntax {
	return ruleLine{vocab.Span{Slot: at.Slot, From: at.Fact.Outer.From.At, To: at.Fact.Outer.To.At}}
}

type ruleLine struct{ span vocab.Span }

func (l ruleLine) Syntax() vocab.Span { return l.span }

// blockConfig is which block prefixes the analyzer recognises.
type blockConfig struct {
	Headings, Lists, Quotes, Code, Math, Rules, Tables bool
}

// analyzeBlocks finds a paragraph's block marker and fence.
func analyzeBlocks(cfg blockConfig, mk *policy.FactKind[marker], fk *policy.FactKind[fence], sk *policy.FactKind[rule]) func(u *policy.Unit, e *policy.Emit) {
	return func(u *policy.Unit, e *policy.Emit) {
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
				e.Fact(mk.Span(0, len(profile.TaskOpenSpelling), marker{Kind: markTaskOpen, Task: profile.TaskOpen}))
			case strings.HasPrefix(t, string(profile.TaskDoneSpelling)),
				strings.HasPrefix(t, string(profile.TaskDoneAltSpelling)):
				e.Fact(mk.Span(0, len(profile.TaskDoneSpelling), marker{Kind: markTaskDone, Task: profile.TaskDone}))
			}
			return
		}
		headingChar := profile.HeadingMarker[0]
		quoteChar := profile.QuoteSpelling[0]
		switch {
		case cfg.Headings && t[0] == headingChar:
			n := 0
			for n < len(t) && t[n] == headingChar {
				n++
			}
			// A heading's hashes end at a space or the text's end; one
			// a letter follows starts a tag.
			if n <= 6 && (n == len(t) || t[n] == ' ') {
				e.Fact(mk.Span(0, n, marker{Kind: markHeading, Level: n}))
			}
		case cfg.Quotes && t[0] == quoteChar:
			e.Fact(mk.Span(0, 1, marker{Kind: markQuote}))
		case cfg.Lists && (t[0] == profile.BulletSpellings[0][0] || t[0] == profile.BulletSpellings[1][0] || t[0] == profile.BulletSpellings[2][0]):
			e.Fact(mk.Span(0, 1, marker{Kind: markBullet, Char: t[0]}))
		case cfg.Lists && t[0] >= '0' && t[0] <= '9':
			n := 0
			for n < len(t) && n < 9 && t[n] >= '0' && t[n] <= '9' {
				n++
			}
			if n < len(t) && (t[n] == profile.OrderedDelimiters[0] || t[n] == profile.OrderedDelimiters[1]) {
				start, _ := profile.ParseOrdered(t[:n] + ".")
				e.Fact(mk.Span(0, n+1, marker{Kind: markOrdered, Level: start, Char: t[n]}))
			}
		}
		switch {
		case cfg.Code && strings.HasPrefix(t, string(profile.FenceSpelling)) && !strings.Contains(t[3:], "`"):
			e.Fact(fk.Span(0, len(t), fence{Kind: fenceCode, Info: strings.TrimSpace(t[3:])}))
		case cfg.Math && t == string(profile.MathSpelling):
			e.Fact(fk.Span(0, len(t), fence{Kind: fenceMath}))
		case cfg.Rules && len(t) >= 3 && ruleLineOf(t):
			e.Fact(fk.Span(0, len(t), fence{Kind: fenceDivider}))
		case cfg.Tables && profile.TableRow(t) != nil:
			e.Fact(fk.Span(0, len(t), fence{Kind: fenceTable, Info: t}))
		}
		if cfg.Rules && len(t) >= 2 && ruleLineOf(t) {
			e.Fact(sk.Span(0, len(t), rule{}))
		}
	}
}

// ruleLineOf reports a line of one rule character.
func ruleLineOf(t string) bool {
	return strings.Count(t, t[:1]) == len(t) && strings.ContainsRune(profile.RuleChars, rune(t[0]))
}

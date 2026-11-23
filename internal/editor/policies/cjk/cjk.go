// Package cjk is the CJK policy: a Script rewrite that refines punctuation
// width by a line's script and spaces adjacent CJK and Latin text. It
// reacts only to text typed or committed by an IME.
package cjk

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "cjk"

// Options.
var (
	// Punctuation gives ASCII punctuation a full width on a CJK line.
	Punctuation = policy.Bool("punctuation", true, policy.Label("settings.cjk.punctuation"), policy.Description("settings.cjk.punctuation.description"))
	// Spacing puts one space between adjacent CJK and Latin text.
	Spacing = policy.Bool("spacing", true, policy.Label("settings.cjk.spacing"), policy.Description("settings.cjk.spacing.description"))
)

var textEdits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}

type options struct {
	punctuation, spacing bool
}

// Policy is the cjk policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	opts := options{
		punctuation: policy.Get(p, Punctuation),
		spacing:     policy.Get(p, Spacing),
	}
	p.Rewrite("script", policy.PhaseScript).
		Trigger(policy.Trigger{
			Kinds:   []doc.RecordKind{doc.RecordTextInserted},
			Classes: []doc.OriginClass{doc.OriginTyping},
		}).
		Extent(policy.ChangedRanges).
		Produces(textEdits...).
		Do(func(c *policy.Ctx, site policy.Site) policy.Result {
			return rewrite(c, site, opts)
		})

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("今天用Go写代码,很开心."),
	)
}, policy.Label("policies.cjk"), policy.Description("policies.cjk.description"))

type span struct{ from, to int }

func rewrite(c *policy.Ctx, site policy.Site, opts options) policy.Result {
	spec, ok := vocab.SlotSpec(c.Doc(), site.Slot)
	if !ok || spec.Literal {
		return policy.Swallow
	}
	in, ok := c.Inline(site.Slot)
	if !ok {
		return policy.Swallow
	}
	flat := in.Flat()
	return policy.Do(vocab.RefineLines(site.Slot, in, site.From, site.To, func(stretch vocab.Runes, at int) (vocab.Runes, bool) {
		return refine(stretch, lineHasCJK(flat, at), opts)
	})...)
}

func lineHasCJK(text string, at int) bool {
	at, end := vocab.LineAround(text, at, at)
	for at < end {
		r, size := utf8.DecodeRuneInString(text[at:])
		if r != doc.NodeByte && vocab.IsCJK(r) {
			return true
		}
		at += size
	}
	return false
}

func refine(cells vocab.Runes, hasCJK bool, opts options) (vocab.Runes, bool) {
	cells = slices.Clone(cells)
	before := cells.Text()
	urls := urlRanges(cells)
	if opts.punctuation && hasCJK {
		cells = punctuationWidth(cells, urls)
		urls = urlRanges(cells)
	}
	if opts.spacing {
		cells = cjkLatinSpacing(cells, urls)
	}
	return cells, cells.Text() != before
}

var fullwidth = map[rune]rune{
	',': '，',
	'.': '。',
	':': '：',
	';': '；',
	'!': '！',
	'?': '？',
	'(': '（',
	')': '）',
}

// punctuationWidth changes ASCII punctuation only outside URLs. A numeric
// separator, and a possible decimal or clock punctuation at a line's end,
// is left for the next character to settle.
func punctuationWidth(cells vocab.Runes, urls []span) vocab.Runes {
	for i, cell := range cells {
		want, ok := fullwidth[cell.R]
		if !ok || inSpans(urls, i) || numericException(cells, i) || deferredException(cells, i) {
			continue
		}
		cells[i].R = want
	}
	return cells
}

func numericException(cells vocab.Runes, i int) bool {
	if i == 0 || i+1 >= len(cells) {
		return false
	}
	return unicode.IsDigit(cells[i-1].R) && unicode.IsDigit(cells[i+1].R)
}

// deferredException keeps punctuation that might start a decimal, time or
// URL intact until the character after it proves which it is.
func deferredException(cells vocab.Runes, i int) bool {
	if i == 0 {
		return false
	}
	if unicode.IsDigit(cells[i-1].R) && i+1 == len(cells) {
		return true
	}
	if cells[i].R != ':' {
		return false
	}
	j := i - 1
	for j >= 0 && schemeRune(cells[j].R) {
		j--
	}
	if j == i-1 || j+1 >= i || !unicode.IsLetter(cells[j+1].R) {
		return false
	}
	return i+1 == len(cells) || cells[i+1].R == '/'
}

// cjkLatinSpacing inserts one space at either direction of CJK/Latin
// adjacency. Punctuation and URL interiors are not boundaries.
func cjkLatinSpacing(cells vocab.Runes, urls []span) vocab.Runes {
	out := make(vocab.Runes, 0, len(cells)+8)
	for i, cell := range cells {
		out = append(out, cell)
		if i+1 >= len(cells) || inSpans(urls, i) && inSpans(urls, i+1) {
			continue
		}
		next := cells[i+1].R
		if cjkLetter(cell.R) && latinLike(next) || latinLike(cell.R) && cjkLetter(next) {
			out = append(out, vocab.StyledRune{R: ' ', Marks: cell.Marks})
		}
	}
	return out
}

// cjkLetter is a CJK letter; latinLike a letter or digit outside CJK.
func cjkLetter(r rune) bool {
	return vocab.ClassOf(r)&(vocab.ClassWord|vocab.ClassDigit|vocab.ClassCJK) == vocab.ClassWord|vocab.ClassCJK
}

func latinLike(r rune) bool {
	return vocab.ClassOf(r)&(vocab.ClassWord|vocab.ClassCJK) == vocab.ClassWord
}

func inSpans(spans []span, at int) bool {
	for _, s := range spans {
		if s.from <= at && at < s.to {
			return true
		}
	}
	return false
}

// urlRanges finds scheme URLs and www addresses. Trailing sentence
// punctuation is left outside the protected range.
func urlRanges(cells vocab.Runes) []span {
	var out []span
	for i := 0; i < len(cells); i++ {
		if i > 0 && schemeRune(cells[i-1].R) {
			continue
		}
		start := i
		if cells.HasPrefix(i, "www.") {
			i += len("www.")
		} else {
			j := i
			for j < len(cells) && schemeRune(cells[j].R) {
				j++
			}
			if j == i {
				continue
			}
			letters := false
			for _, c := range cells[i:j] {
				letters = letters || unicode.IsLetter(c.R)
			}
			if !letters || !cells.HasPrefix(j, "://") {
				continue
			}
			i = j + len("://")
		}
		end := i
		for end < len(cells) && !unicode.IsSpace(cells[end].R) && !vocab.IsCJK(cells[end].R) {
			end++
		}
		if end < len(cells) {
			for end > i && strings.ContainsRune(".,;:!?", cells[end-1].R) {
				end--
			}
		}
		if end > start {
			out = append(out, span{start, end})
		}
		i = end - 1
	}
	return out
}

func schemeRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'
}

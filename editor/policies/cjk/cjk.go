// Package cjk is the CJK policy: a Script rewrite that refines punctuation
// width by a line's script and spaces adjacent CJK and Latin text. It
// reacts only to text typed or committed by an IME.
package cjk

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
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
	p.Rewrite("script", doc.PhaseScript).
		Trigger(policy.Trigger{
			Kinds:   []doc.RecordKind{doc.RecordTextInserted},
			Classes: []doc.OriginClass{doc.OriginTyping},
		}).
		Extent(policy.ChangedRanges).
		Produces(textEdits...).
		Do(func(c *policy.Ctx, site policy.Site) policy.Result {
			return rewrite(c, site, opts)
		})
})

type styledRune struct {
	r     rune
	marks *doc.MarkSet
}

type change struct {
	from, to int
	items    []doc.Item
}

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
	from := lineStart(flat, site.From)
	to := lineEnd(flat, max(site.From, site.To))
	if from >= to {
		return policy.Swallow
	}

	var changes []change
	var stretch []styledRune
	stretchFrom, stretchTo := 0, 0
	flush := func() {
		if len(stretch) == 0 {
			return
		}
		out, changed := refine(stretch, lineHasCJK(flat, stretchFrom), opts)
		if changed {
			si, so := itemsOf(stretch), itemsOf(out)
			ds := vocab.MinimalTextChanges(si, so)
			for _, d := range ds {
				changes = append(changes, change{stretchFrom + d.From, stretchFrom + d.To, d.Items})
			}
		}
		stretch = nil
	}

	in.Each(func(at int, item doc.Item) bool {
		if at >= to {
			return false
		}
		if item.Node != nil {
			flush()
			return true
		}
		lo, hi := max(at, from), min(at+len(item.Text), to)
		if lo >= hi {
			return true
		}
		for off, r := range item.Text {
			abs := at + off
			if abs < lo || abs >= hi {
				continue
			}
			if r == '\n' {
				flush()
				continue
			}
			if len(stretch) > 0 && abs != stretchTo {
				flush()
			}
			if len(stretch) == 0 {
				stretchFrom = abs
			}
			stretch = append(stretch, styledRune{r: r, marks: item.Marks})
			stretchTo = abs + utf8.RuneLen(r)
		}
		return true
	})
	flush()

	if len(changes) == 0 {
		return policy.Swallow
	}
	edits := make([]vocab.Edit, 0, len(changes))
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		edits = append(edits, vocab.Ops(doc.EditText{
			At: site.Slot, From: change.from, To: change.to, Items: change.items,
		}))
	}
	return policy.Do(edits...)
}

func lineStart(text string, at int) int {
	at = max(0, min(at, len(text)))
	for at > 0 && text[at-1] != '\n' {
		at--
	}
	return at
}

func lineEnd(text string, at int) int {
	at = max(0, min(at, len(text)))
	for at < len(text) && text[at] != '\n' {
		at++
	}
	return at
}

func lineHasCJK(text string, at int) bool {
	at = lineStart(text, at)
	for at < len(text) && text[at] != '\n' {
		r, size := utf8.DecodeRuneInString(text[at:])
		if r != doc.NodeByte && vocab.IsCJK(r) {
			return true
		}
		at += size
	}
	return false
}

func refine(cells []styledRune, hasCJK bool, opts options) ([]styledRune, bool) {
	cells = slices.Clone(cells)
	before := cellsText(cells)
	urls := urlRanges(cells)
	if opts.punctuation && hasCJK {
		cells = punctuationWidth(cells, urls)
		urls = urlRanges(cells)
	}
	if opts.spacing {
		cells = cjkLatinSpacing(cells, urls)
	}
	return cells, cellsText(cells) != before
}

func itemsOf(cells []styledRune) []doc.Item {
	var out []doc.Item
	for i := 0; i < len(cells); {
		marks := cells[i].marks
		var b strings.Builder
		for i < len(cells) && cells[i].marks == marks {
			b.WriteRune(cells[i].r)
			i++
		}
		out = append(out, doc.TextItem(b.String(), marks))
	}
	return out
}

func cellsText(cells []styledRune) string {
	var b strings.Builder
	for _, cell := range cells {
		b.WriteRune(cell.r)
	}
	return b.String()
}

func refineText(text string, opts options) string {
	cells := make([]styledRune, 0, utf8.RuneCountInString(text))
	hasCJK := false
	for _, r := range text {
		cells = append(cells, styledRune{r: r})
		hasCJK = hasCJK || vocab.IsCJK(r)
	}
	out, _ := refine(cells, hasCJK, opts)
	var b strings.Builder
	for _, cell := range out {
		b.WriteRune(cell.r)
	}
	return b.String()
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
func punctuationWidth(cells []styledRune, urls []span) []styledRune {
	for i, cell := range cells {
		want, ok := fullwidth[cell.r]
		if !ok || inSpans(urls, i) || numericException(cells, i) || deferredException(cells, i) {
			continue
		}
		cells[i].r = want
	}
	return cells
}

func numericException(cells []styledRune, i int) bool {
	if i == 0 || i+1 >= len(cells) {
		return false
	}
	return unicode.IsDigit(cells[i-1].r) && unicode.IsDigit(cells[i+1].r)
}

// deferredException keeps punctuation that might start a decimal, time or
// URL intact until the character after it proves which it is.
func deferredException(cells []styledRune, i int) bool {
	if i == 0 {
		return false
	}
	if unicode.IsDigit(cells[i-1].r) && i+1 == len(cells) {
		return true
	}
	if cells[i].r != ':' {
		return false
	}
	j := i - 1
	for j >= 0 && schemeRune(cells[j].r) {
		j--
	}
	if j == i-1 || j+1 >= i || !unicode.IsLetter(cells[j+1].r) {
		return false
	}
	return i+1 == len(cells) || cells[i+1].r == '/'
}

// cjkLatinSpacing inserts one space at either direction of CJK/Latin
// adjacency. Punctuation and URL interiors are not boundaries.
func cjkLatinSpacing(cells []styledRune, urls []span) []styledRune {
	out := make([]styledRune, 0, len(cells)+8)
	for i, cell := range cells {
		out = append(out, cell)
		if i+1 >= len(cells) || inSpans(urls, i) && inSpans(urls, i+1) {
			continue
		}
		next := cells[i+1].r
		if cjkLetter(cell.r) && latinLike(next) || latinLike(cell.r) && cjkLetter(next) {
			out = append(out, styledRune{r: ' ', marks: cell.marks})
		}
	}
	return out
}

func cjkLetter(r rune) bool {
	return vocab.IsCJK(r) && unicode.IsLetter(r)
}

func latinLike(r rune) bool {
	return (unicode.IsLetter(r) || unicode.IsDigit(r)) && !vocab.IsCJK(r)
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
func urlRanges(cells []styledRune) []span {
	var out []span
	for i := 0; i < len(cells); i++ {
		if i > 0 && schemeRune(cells[i-1].r) {
			continue
		}
		start := i
		if prefix(cells, i, "www.") {
			i += len("www.")
		} else {
			j := i
			for j < len(cells) && schemeRune(cells[j].r) {
				j++
			}
			if j == i {
				continue
			}
			letters := false
			for _, c := range cells[i:j] {
				letters = letters || unicode.IsLetter(c.r)
			}
			if !letters || !prefix(cells, j, "://") {
				continue
			}
			i = j + len("://")
		}
		end := i
		for end < len(cells) && !unicode.IsSpace(cells[end].r) && !vocab.IsCJK(cells[end].r) {
			end++
		}
		if end < len(cells) {
			for end > i && strings.ContainsRune(".,;:!?", cells[end-1].r) {
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

func prefix(cells []styledRune, at int, text string) bool {
	for _, want := range text {
		if at >= len(cells) || cells[at].r != want {
			return false
		}
		at++
	}
	return true
}

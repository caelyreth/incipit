// Package typography is the typography policy: a Characters rewrite that
// replaces ellipses and dashes, spaces punctuation, curls apostrophes and
// places terminal punctuation inside a closing quote. It reacts only to
// text typed or committed by an IME, never to paste, commands, agents,
// history or timers.
package typography

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
const ID = "typography"

// Options.
var (
	// Symbols replaces ellipses and doubled hyphens with typographic
	// characters.
	Symbols = policy.Bool("symbols", true, policy.Label("settings.typography.symbols"), policy.Description("settings.typography.symbols.description"))
	// Punctuation spaces punctuation as it is typed.
	Punctuation = policy.Bool("punctuation", true, policy.Label("settings.typography.punctuation"), policy.Description("settings.typography.punctuation.description"))
	// QuotePlacement curls apostrophes and moves terminal punctuation
	// inside a closing quote.
	QuotePlacement = policy.Bool("quotePlacement", true, policy.Label("settings.typography.quotePlacement"), policy.Description("settings.typography.quotePlacement.description"))
)

var textEdits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted}

type options struct {
	symbols, punctuation, quotes bool
}

// Policy is the typography policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	opts := options{
		symbols:     policy.Get(p, Symbols),
		punctuation: policy.Get(p, Punctuation),
		quotes:      policy.Get(p, QuotePlacement),
	}
	p.Rewrite("characters", doc.PhaseCharacters).
		Trigger(policy.Trigger{
			Kinds:   []doc.RecordKind{doc.RecordTextInserted},
			Classes: []doc.OriginClass{doc.OriginTyping, doc.OriginIME},
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
	if dividerLine(flat[from:to]) {
		return policy.Swallow
	}

	var changes []change
	var stretch []styledRune
	stretchFrom, stretchTo := 0, 0
	flush := func() {
		if len(stretch) == 0 {
			return
		}
		out, changed := rewriteCells(stretch, opts)
		if changed {
			for _, d := range vocab.MinimalTextChanges(itemsOf(stretch), itemsOf(out)) {
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

// dividerLine reports a line that may become a Markdown divider. Typography
// leaves its dashes alone so the divider reaches the Structure phase intact.
func dividerLine(text string) bool {
	if len(text) < 2 {
		return false
	}
	for _, r := range text {
		if r != '-' {
			return false
		}
	}
	return true
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

func rewriteText(text string, opts options) string {
	cells := make([]styledRune, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		cells = append(cells, styledRune{r: r})
	}
	out, _ := rewriteCells(cells, opts)
	var b strings.Builder
	for _, cell := range out {
		b.WriteRune(cell.r)
	}
	return b.String()
}

func rewriteCells(cells []styledRune, opts options) ([]styledRune, bool) {
	cells = slices.Clone(cells)
	before := cellsText(cells)
	if opts.symbols {
		cells = symbols(cells)
	}
	if opts.punctuation {
		cells = spacePunctuation(cells)
	}
	if opts.quotes {
		apostrophes(cells)
		punctuationInsideQuotes(cells)
	}
	return cells, cellsText(cells) != before
}

func cellsText(cells []styledRune) string {
	var b strings.Builder
	for _, cell := range cells {
		b.WriteRune(cell.r)
	}
	return b.String()
}

// symbols replaces each three periods with an ellipsis and two hyphens
// with an em dash. The trailing "—-" rule makes a third hyphen typed
// after the rewrite's second land at the same em dash.
func symbols(cells []styledRune) []styledRune {
	out := make([]styledRune, 0, len(cells))
	for i := 0; i < len(cells); {
		n, replacement := 0, rune(0)
		switch {
		case prefix(cells, i, "..."):
			n, replacement = 3, '…'
		case prefix(cells, i, "---"):
			n, replacement = 3, '—'
		case prefix(cells, i, "--"):
			n, replacement = 2, '—'
		case prefix(cells, i, "—-"):
			n, replacement = 2, '—'
		}
		if n == 0 {
			out = append(out, cells[i])
			i++
			continue
		}
		out = append(out, styledRune{r: replacement, marks: cells[i].marks})
		i += n
	}
	return out
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

// apostrophes curls a straight apostrophe between letters or digits.
func apostrophes(cells []styledRune) {
	for i, cell := range cells {
		if cell.r != '\'' || i == 0 || i+1 >= len(cells) {
			continue
		}
		if isWordRune(cells[i-1].r) && isWordRune(cells[i+1].r) {
			cells[i].r = '’'
		}
	}
}

// punctuationInsideQuotes moves a terminal period or comma typed right
// after a closing quote to before it.
func punctuationInsideQuotes(cells []styledRune) {
	for i := 0; i+1 < len(cells); i++ {
		if cells[i].r != '"' && cells[i].r != '”' {
			continue
		}
		if cells[i+1].r != '.' && cells[i+1].r != ',' {
			continue
		}
		if i == 0 || unicode.IsSpace(cells[i-1].r) || isOpener(cells[i-1].r) {
			continue
		}
		if i+2 < len(cells) && !unicode.IsSpace(cells[i+2].r) {
			continue
		}
		cells[i], cells[i+1] = cells[i+1], cells[i]
		i++
	}
}

func isOpener(r rune) bool {
	return strings.ContainsRune("([{“‘«「『（【《〈〔［｛", r)
}

// spacePunctuation removes horizontal space before terminal punctuation
// and supplies one after a comma, semicolon or colon when a Latin letter
// follows immediately. Decimals, times and URL syntax therefore keep their
// own spacing.
func spacePunctuation(cells []styledRune) []styledRune {
	trimmed := make([]styledRune, 0, len(cells))
	for i := 0; i < len(cells); {
		if cells[i].r != ' ' && cells[i].r != '\t' {
			trimmed = append(trimmed, cells[i])
			i++
			continue
		}
		j := i + 1
		for j < len(cells) && (cells[j].r == ' ' || cells[j].r == '\t') {
			j++
		}
		if j < len(cells) && strings.ContainsRune(",.;:!?", cells[j].r) {
			i = j
			continue
		}
		trimmed = append(trimmed, cells[i:j]...)
		i = j
	}

	out := make([]styledRune, 0, len(trimmed)+4)
	for i, cell := range trimmed {
		out = append(out, cell)
		if cell.r != ',' && cell.r != ';' && cell.r != ':' {
			continue
		}
		if i+1 == len(trimmed) || unicode.IsSpace(trimmed[i+1].r) {
			continue
		}
		if isLatinLetter(trimmed[i+1].r) {
			out = append(out, styledRune{r: ' ', marks: cell.marks})
		}
	}
	return out
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isLatinLetter(r rune) bool {
	return unicode.IsLetter(r) && !vocab.IsCJK(r)
}

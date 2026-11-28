// Package typography is the typography policy: a Characters rewrite that
// replaces ellipses and dashes, spaces punctuation, curls apostrophes and
// places terminal punctuation inside a closing quote. It reacts only to
// text typed or committed by an IME, never to paste, commands, agents,
// history or timers.
package typography

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
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
	p.Rewrite("characters", policy.PhaseCharacters).
		Trigger(policy.Trigger{
			Kinds:   []doc.RecordKind{doc.RecordTextInserted},
			Classes: []doc.OriginClass{doc.OriginTyping},
		}).
		Extent(policy.ChangedRanges).
		// What spells structure still to come, as a rule's dashes do, is
		// its policy's text.
		Spares(vocab.RoleSyntax).
		Produces(textEdits...).
		Do(func(c *policy.Ctx, site policy.Site) policy.Result {
			return rewrite(c, site, opts)
		})

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("Wait... it's done -- almost."),
	)
}, policy.Label("policies.typography"), policy.Description("policies.typography.description"))

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
	if from, to := vocab.LineAround(flat, site.From, site.To); from >= to {
		return policy.Swallow
	}
	return policy.Do(vocab.RefineLines(c.Doc().Schema(), site.Slot, in, site.From, site.To, func(stretch vocab.Runes, at int) (vocab.Runes, bool) {
		return rewriteCells(stretch, opts)
	})...)
}

func rewriteCells(cells vocab.Runes, opts options) (vocab.Runes, bool) {
	cells = slices.Clone(cells)
	before := cells.Text()
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
	return cells, cells.Text() != before
}

// symbols replaces each three periods with an ellipsis and two hyphens
// with an em dash. The trailing "—-" rule makes a third hyphen typed
// after the rewrite's second land at the same em dash.
func symbols(cells vocab.Runes) vocab.Runes {
	out := make(vocab.Runes, 0, len(cells))
	for i := 0; i < len(cells); {
		n, replacement := 0, rune(0)
		switch {
		case cells.HasPrefix(i, "..."):
			n, replacement = 3, '…'
		case cells.HasPrefix(i, "---"):
			n, replacement = 3, '—'
		case cells.HasPrefix(i, "--"):
			n, replacement = 2, '—'
		case cells.HasPrefix(i, "—-"):
			n, replacement = 2, '—'
		}
		if n == 0 {
			out = append(out, cells[i])
			i++
			continue
		}
		out = append(out, vocab.StyledRune{R: replacement, Marks: cells[i].Marks})
		i += n
	}
	return out
}

// apostrophes curls a straight apostrophe between letters or digits.
func apostrophes(cells vocab.Runes) {
	for i, cell := range cells {
		if cell.R != '\'' || i == 0 || i+1 >= len(cells) {
			continue
		}
		if isWordRune(cells[i-1].R) && isWordRune(cells[i+1].R) {
			cells[i].R = '’'
		}
	}
}

// punctuationInsideQuotes moves a terminal period or comma typed right
// after a closing quote to before it.
func punctuationInsideQuotes(cells vocab.Runes) {
	for i := 0; i+1 < len(cells); i++ {
		if cells[i].R != '"' && cells[i].R != '”' {
			continue
		}
		if cells[i+1].R != '.' && cells[i+1].R != ',' {
			continue
		}
		if i == 0 || unicode.IsSpace(cells[i-1].R) || isOpener(cells[i-1].R) {
			continue
		}
		if i+2 < len(cells) && !unicode.IsSpace(cells[i+2].R) {
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
func spacePunctuation(cells vocab.Runes) vocab.Runes {
	trimmed := make(vocab.Runes, 0, len(cells))
	for i := 0; i < len(cells); {
		if cells[i].R != ' ' && cells[i].R != '\t' {
			trimmed = append(trimmed, cells[i])
			i++
			continue
		}
		j := i + 1
		for j < len(cells) && (cells[j].R == ' ' || cells[j].R == '\t') {
			j++
		}
		if j < len(cells) && strings.ContainsRune(",.;:!?", cells[j].R) {
			i = j
			continue
		}
		trimmed = append(trimmed, cells[i:j]...)
		i = j
	}

	out := make(vocab.Runes, 0, len(trimmed)+4)
	for i, cell := range trimmed {
		out = append(out, cell)
		if cell.R != ',' && cell.R != ';' && cell.R != ':' {
			continue
		}
		if i+1 == len(trimmed) || unicode.IsSpace(trimmed[i+1].R) {
			continue
		}
		if isLatinLetter(trimmed[i+1].R) {
			out = append(out, vocab.StyledRune{R: ' ', Marks: cell.Marks})
		}
	}
	return out
}

// isWordRune is a letter or digit.
func isWordRune(r rune) bool { return vocab.ClassOf(r)&vocab.ClassWord != 0 }

// isLatinLetter is a letter outside CJK.
func isLatinLetter(r rune) bool {
	return vocab.ClassOf(r)&(vocab.ClassWord|vocab.ClassDigit|vocab.ClassCJK) == vocab.ClassWord
}

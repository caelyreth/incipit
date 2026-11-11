// Package statistics is the statistics policy: it summarises a page's
// words, characters and CJK characters, and counts what a selection
// covers by the same rules.
package statistics

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Counts are one stretch of text's statistics.
type Counts struct {
	Words      int
	Characters int
	CJK        int
}

// Summary is the page summary the compiled policy exposes. It is assigned
// when the policy is built.
var Summary *policy.SummaryKind[Counts]

// Policy is the statistics policy.
var Policy = policy.Define("statistics", func(p *policy.P) {
	Summary = policy.Summarize(p, "page", policy.Summary[Counts]{
		Leaf:     leaf,
		Combine:  combine,
		Identity: Counts{},
	})
}, policy.Label("policies.statistics"), policy.Description("policies.statistics.description"))

func combine(a, b Counts) Counts {
	return Counts{Words: a.Words + b.Words, Characters: a.Characters + b.Characters, CJK: a.CJK + b.CJK}
}

// leaf counts the node's own inline text. Inline node children are separate
// nodes, which the summary combines; Inline.Text leaves them out.
func leaf(n *doc.Node) Counts {
	var c Counts
	for _, slot := range n.Slots() {
		if slot.IsInline() {
			c = combine(c, CountText(slot.Content().Text()))
		}
	}
	return c
}

// CountsOf counts a whole node tree directly, for callers that have no
// compiled program yet.
func CountsOf(n *doc.Node) Counts {
	if n == nil {
		return Counts{}
	}
	c := leaf(n)
	for _, slot := range n.Slots() {
		if slot.IsInline() {
			slot.Content().Nodes(func(_ int, child *doc.Node) bool {
				c = combine(c, CountsOf(child))
				return true
			})
			continue
		}
		for _, child := range slot.Children() {
			c = combine(c, CountsOf(child))
		}
	}
	return c
}

// Page counts a page, using the policy's memoised summary once the
// statistics policy is part of the compiled program.
func Page(store *policy.Store, root *doc.Node) Counts {
	if store != nil && Summary != nil {
		return policy.SummaryOf(store, Summary, root)
	}
	return CountsOf(root)
}

// CountText counts a stretch of plain text, as a selection or a page's
// inline text. Line breaks separate blocks and are not characters.
func CountText(text string) Counts {
	var c Counts
	for _, r := range text {
		if r == '\n' {
			continue
		}
		c.Characters++
		if vocab.IsCJK(r) {
			c.CJK++
		}
	}
	for segment := range vocab.WordSegments(text) {
		if segment.WordLike {
			c.Words++
		}
	}
	return c
}

// Format is the footer's spelling of the counts. A selection is shown
// against the page's totals; nil means nothing is selected.
func Format(page Counts, selection *Counts) string {
	if selection == nil {
		return formatCounts(page)
	}
	parts := []string{
		fmt.Sprintf("%s of %s", count(selection.Words, "word"), count(page.Words, "word")),
		fmt.Sprintf("%s of %s", count(selection.Characters, "character"), count(page.Characters, "character")),
	}
	if page.CJK > 0 || selection.CJK > 0 {
		parts = append(parts, fmt.Sprintf("%s of %s", number(selection.CJK), number(page.CJK)+" CJK"))
	}
	return strings.Join(parts, "  ·  ")
}

func formatCounts(c Counts) string {
	parts := []string{count(c.Words, "word"), count(c.Characters, "character")}
	if c.CJK > 0 {
		parts = append(parts, number(c.CJK)+" CJK")
	}
	return strings.Join(parts, "  ·  ")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return number(n) + " " + noun + "s"
}

// number groups thousands with commas, as the library's counts do.
func number(n int) string {
	digits := strconv.Itoa(n)
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	lead := len(digits) % 3
	if lead > 0 {
		b.WriteString(digits[:lead])
	}
	for i := lead; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

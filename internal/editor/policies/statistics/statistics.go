// Package statistics is the statistics policy: it summarises a page's
// words, characters and CJK characters, and counts what a selection
// covers by the same rules.
package statistics

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
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
	c.Words = vocab.CountWords(text)
	return c
}

package search

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
)

// Line is the line of text that holds the first of matches (Find), cut to
// about width characters around it, with the matches it shows; with no
// match, the text's first line.
func Line(text string, matches []Span, width int) (string, []Span) {
	at := 0
	if len(matches) > 0 {
		at = matches[0].From
	}
	from := strings.LastIndexByte(text[:at], '\n') + 1
	to := len(text)
	if i := strings.IndexByte(text[at:], '\n'); i >= 0 {
		to = at + i
	}
	var in []Span
	for _, m := range matches {
		if m.From >= from && m.To <= to {
			in = append(in, Span{m.From - from, m.To - from})
		}
	}
	return Snippet(text[from:to], in, width)
}

// Spot is a match in a page: the inline slot it is in, and its byte
// range there as doc.Pos counts it, an inline node counting one byte.
type Spot struct {
	Slot doc.SlotRef
	Span Span
}

// FirstMatch is where the matcher first finds its words in a page, in
// reading order. Formulas' sources are not read, as the index does not
// read them.
func FirstMatch(root *doc.Node, m *Matcher) (Spot, bool) {
	var spot Spot
	found := false
	var visit func(n *doc.Node)
	visit = func(n *doc.Node) {
		if found || n.Kind() == profile.Math {
			return
		}
		for i, slot := range n.Slots() {
			if found {
				return
			}
			if !slot.IsInline() {
				for _, c := range slot.Children() {
					visit(c)
				}
				continue
			}
			var text strings.Builder
			slot.Content().Each(func(_ int, it doc.Item) bool {
				if it.Node != nil {
					text.WriteByte(' ')
				} else {
					text.WriteString(it.Text)
				}
				return true
			})
			if spans := m.Find(text.String()); len(spans) > 0 {
				spot, found = Spot{Slot: doc.SlotRef{Node: n.ID(), Slot: i}, Span: spans[0]}, true
			}
		}
	}
	visit(root)
	return spot, found
}

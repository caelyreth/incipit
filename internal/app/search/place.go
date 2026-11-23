package search

import (
	"github.com/caelyreth/neverland/internal/text/query"

	"strings"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
)

// Spot is a match in a page: the inline slot it is in, and its byte
// range there as doc.Pos counts it, an inline node counting one byte.
type Spot struct {
	Slot doc.SlotRef
	Span query.Span
}

// FirstMatch is where the matcher first finds its words in a page, in
// reading order. Formulas' sources are not read, as the index does not
// read them.
func FirstMatch(root *doc.Node, m *query.Matcher) (Spot, bool) {
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

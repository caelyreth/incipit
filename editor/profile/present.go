package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Contain: quotes and lists open their slots to the units inside; an item
// is a unit carrying its marker and opens its body.
func (product) Contain(r doc.Reader, n *doc.Node, outer vocab.Container) vocab.Contained {
	switch n.Kind() {
	case Quote:
		inner := outer
		inner.Quotes++
		inner.Marker = nil
		return vocab.Contained{Open: true, Inner: inner}
	case List:
		inner := outer
		inner.Depth++
		inner.Indent++
		inner.Marker = nil
		return vocab.Contained{Open: true, Inner: inner}
	case Item:
		own := outer
		mk, _ := MarkerOf(r, n)
		own.Marker = &vocab.ItemMarker{Item: n.ID(), Data: mk}
		return vocab.Contained{Unit: true, Own: own, Open: true, Inner: outer}
	}
	return vocab.Contained{Unit: true, Own: outer}
}

// Style is a kind's group and accessible role; text in a quote reads as
// quoted.
func (product) Style(r doc.Reader, n *doc.Node, ctx vocab.Container) vocab.Style {
	s := vocab.Style{Group: vocab.GroupText, Role: "paragraph"}
	switch n.Kind() {
	case Heading:
		s.Level = HeadingLevel(n)
		s.Group, s.Role = GroupHeadings[s.Level], "heading"
	case Item:
		s.Role = "listitem"
	case Code:
		s.Group, s.Role = GroupCode, "code"
	case Math:
		s.Group, s.Role = GroupFormulaSource, "math"
	case Formula:
		s.Group, s.Role = GroupFormulaSource, "math"
	case Divider:
		s.Role = "separator"
	case Figure:
		s.Group, s.Role = GroupCaption, "figure"
	case Table:
		s.Group, s.Role = GroupTable, "table"
	case Cell:
		s.Group, s.Role = GroupCell, "cell"
		// A header row's cells head their columns.
		if g, at, ok := vocab.Locate(r, n.ID()); ok && at.Row == 0 && TableShapeOf(g.Table).Header {
			s.Group, s.Role = GroupCellHeader, "columnheader"
		}
	}
	if ctx.Quotes > 0 && s.Group == vocab.GroupText {
		s.Group = GroupQuote
	}
	return s
}

func (product) MarkGroup(k doc.MarkKind) vocab.Group { return MarkGroup(k) }

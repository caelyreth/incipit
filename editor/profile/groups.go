package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The highlight groups of the profile's kinds and marks.
var (
	GroupHeading  = vocab.NewGroup("heading", vocab.GroupText)
	GroupHeadings = [7]vocab.Group{0,
		vocab.NewGroup("heading.1", GroupHeading), vocab.NewGroup("heading.2", GroupHeading), vocab.NewGroup("heading.3", GroupHeading),
		vocab.NewGroup("heading.4", GroupHeading), vocab.NewGroup("heading.5", GroupHeading), vocab.NewGroup("heading.6", GroupHeading)}
	GroupEmphasis      = vocab.NewGroup("mark.emphasis", 0)
	GroupStrong        = vocab.NewGroup("mark.strong", 0)
	GroupCodeMark      = vocab.NewGroup("mark.code", 0)
	GroupStrike        = vocab.NewGroup("mark.strike", 0)
	GroupCode          = vocab.NewGroup("code", vocab.GroupText)
	GroupFormulaSource = vocab.NewGroup("formula.source", GroupCode)
	GroupQuote         = vocab.NewGroup("quote", vocab.GroupText)
	GroupListMarker    = vocab.NewGroup("list.marker", vocab.GroupGhost)
	GroupCaption       = vocab.NewGroup("caption", vocab.GroupText)
	GroupCell          = vocab.NewGroup("cell", vocab.GroupText)
	GroupCellHeader    = vocab.NewGroup("cell.header", GroupCell)
)

// MarkGroup is the group of a mark kind.
func MarkGroup(k doc.MarkKind) vocab.Group {
	switch k {
	case Emphasis:
		return GroupEmphasis
	case Strong:
		return GroupStrong
	case CodeMark:
		return GroupCodeMark
	case Strike:
		return GroupStrike
	}
	return 0
}

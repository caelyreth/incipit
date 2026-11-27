package vocab

import "github.com/caelyreth/incipit/internal/editor/doc"

// Presentation is what a profile tells presentation about its kinds, so
// presentation names none (§13). A profile that does not implement it is
// presented plainly: every block a text unit.
type Presentation interface {
	// Contain is what block n adds in a block slot whose context is outer.
	// It reads only n's kind and attributes and the blocks around it, not
	// text: the units it lists are kept while the page's shape stands
	// (doc.Reader.Shape).
	Contain(r doc.Reader, n *doc.Node, outer Container) Contained
	// Style is the look of a unit, of a text an object holds, or of an
	// inline node's card, in ctx.
	Style(r doc.Reader, n *doc.Node, ctx Container) Style
	// MarkGroup is the group of a mark kind; zero for none.
	MarkGroup(k doc.MarkKind) Group
}

// Container is a unit's container context: what the blocks around it add.
type Container struct {
	// Quotes is how many quotes enclose the unit.
	Quotes int
	// Depth is the list nesting depth; zero outside lists.
	Depth int
	// Marker is set on a unit that carries a list item's marker.
	Marker *ItemMarker
	// Indent is the unit's body indentation, in levels.
	Indent int
}

// ItemMarker is a list item's marker: the item, and the marker as the
// profile spells it, a comparable value.
type ItemMarker struct {
	Item doc.ID
	Data any
}

// Contained is what a block is in the unit walk.
type Contained struct {
	// Unit is set on a block presented as a unit, in context Own.
	Unit bool
	Own  Container
	// Open is set on a block whose block slots hold units, in context Inner.
	Open  bool
	Inner Container
}

// Style is a unit's look: its group, and what assistive technology reads
// it as.
type Style struct {
	Group Group
	Role  string
	Level int
}

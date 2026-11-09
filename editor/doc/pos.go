package doc

import "fmt"

// SlotRef names one slot of one node.
type SlotRef struct {
	Node ID
	Slot int
}

func (s SlotRef) String() string { return fmt.Sprintf("%s.%d", s.Node, s.Slot) }

// Pos is a document position: in an inline slot, a UTF-8 byte offset with
// each inline node counting as one; in a block slot, a gap between
// children.
type Pos struct {
	Node   ID
	Slot   int
	Offset int
}

// At is the position offset in slot s.
func At(s SlotRef, offset int) Pos { return Pos{Node: s.Node, Slot: s.Slot, Offset: offset} }

// Ref is the slot the position is in.
func (p Pos) Ref() SlotRef { return SlotRef{Node: p.Node, Slot: p.Slot} }

func (p Pos) String() string { return fmt.Sprintf("%s.%d@%d", p.Node, p.Slot, p.Offset) }

// Caret is a position and the stop chosen among those the presentation
// defines there. Stop is 0 where a position has a single stop.
type Caret struct {
	Pos
	Stop int
}

// CaretAt is the caret at a position's own stop, 0: where it stands as
// text, the marks it inherits deciding what is typed there (§4.2).
func CaretAt(p Pos) Caret { return Caret{Pos: p} }

// Bias says which side a position at an edit point keeps: before inserted
// content, or after it.
type Bias int8

const (
	BiasBefore Bias = -1
	BiasAfter  Bias = 1
)

// SelectionKind is what a selection selects.
type SelectionKind uint8

const (
	SelectText SelectionKind = iota
	// SelectNode selects a whole object, such as a figure.
	SelectNode
	// SelectCells selects a rectangle of a table's cells.
	SelectCells
)

// CellCoord is a cell by row and column.
type CellCoord struct{ Row, Column int }

// CellRange is a rectangle of a table's cells between an anchor and a
// head, or whole rows or columns of it.
type CellRange struct {
	Table        ID
	Anchor, Head CellCoord
	WholeRows    bool
	WholeColumns bool
}

// Selection is a view's selection. A text selection keeps its direction.
type Selection struct {
	Kind         SelectionKind
	Anchor, Head Caret
	Node         ID
	Cells        CellRange
}

// TextRange selects from anchor to head.
func TextRange(anchor, head Caret) Selection {
	return Selection{Kind: SelectText, Anchor: anchor, Head: head}
}

// Collapsed is a caret with nothing selected.
func Collapsed(c Caret) Selection { return Selection{Kind: SelectText, Anchor: c, Head: c} }

// NodeSelection selects the object id.
func NodeSelection(id ID) Selection { return Selection{Kind: SelectNode, Node: id} }

// CellSelection selects cells.
func CellSelection(r CellRange) Selection { return Selection{Kind: SelectCells, Cells: r} }

// IsCollapsed reports a text selection that selects nothing.
func (s Selection) IsCollapsed() bool { return s.Kind == SelectText && s.Anchor == s.Head }

// Ordered returns a text selection's ends in document order.
func (s Selection) Ordered(r Reader) (from, to Caret) {
	if Compare(r, s.Anchor, s.Head) <= 0 {
		return s.Anchor, s.Head
	}
	return s.Head, s.Anchor
}

// orderKey is a position's place in tree order: for each step from the
// root, the slot and the index or offset there.
func orderKey(r Reader, p Pos) []int {
	key := []int{p.Offset, p.Slot}
	id := p.Node
	for {
		parent, slot, ok := r.Parent(id)
		if !ok {
			break
		}
		pn, _ := r.Node(parent)
		n, _ := r.Node(id)
		var index int
		if s := pn.slots[slot]; s.inline {
			index, _ = s.Content().NodeOffset(id)
		} else {
			index, _ = s.IndexOf(n)
		}
		key = append(key, index, slot)
		id = parent
	}
	for i, j := 0, len(key)-1; i < j; i, j = i+1, j-1 {
		key[i], key[j] = key[j], key[i]
	}
	return key
}

// Compare orders carets by tree order, then offset, then stop. A gap
// before a child comes before every position inside it.
func Compare(r Reader, a, b Caret) int {
	if a.Node == b.Node && a.Slot == b.Slot {
		if a.Offset != b.Offset {
			return a.Offset - b.Offset
		}
		return a.Stop - b.Stop
	}
	ka, kb := orderKey(r, a.Pos), orderKey(r, b.Pos)
	for i := 0; i < len(ka) && i < len(kb); i++ {
		if ka[i] != kb[i] {
			return ka[i] - kb[i]
		}
	}
	if len(ka) != len(kb) {
		return len(ka) - len(kb)
	}
	return a.Stop - b.Stop
}

// Mapper carries positions through a change.
type Mapper interface {
	// Map returns where p is after the change, and false when the content
	// holding p was removed; p is then mapped to where that content was.
	Map(p Pos, bias Bias) (Pos, bool)
}

// MapCaret maps a caret, keeping its stop; the stop is normalised later.
func MapCaret(m Mapper, c Caret, bias Bias) (Caret, bool) {
	p, ok := m.Map(c.Pos, bias)
	if p != c.Pos {
		c.Stop = 0
	}
	c.Pos = p
	return c, ok
}

// MapSelection maps a selection through m. A node selection whose node is
// gone becomes a caret where it was.
func MapSelection(m Mapper, s Selection) Selection {
	switch s.Kind {
	case SelectText:
		bias := BiasAfter
		s.Anchor, _ = MapCaret(m, s.Anchor, bias)
		s.Head, _ = MapCaret(m, s.Head, bias)
	case SelectNode:
		if p, ok := m.Map(Pos{Node: s.Node}, BiasAfter); !ok {
			return Collapsed(CaretAt(p))
		}
	case SelectCells:
		if p, ok := m.Map(Pos{Node: s.Cells.Table}, BiasAfter); !ok {
			return Collapsed(CaretAt(p))
		}
	}
	return s
}

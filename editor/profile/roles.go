package profile

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Profile is the product profile as the framework reads it.
var Profile vocab.Profile = product{}

type product struct{}

func (product) Schema() *doc.Schema             { return Schema() }
func (product) Roles(k doc.Kind) []vocab.RoleID { return NodeRoles(k) }
func (product) Role(r doc.Reader, n *doc.Node, role vocab.RoleID) (any, bool) {
	return NodeRole(r, n, role)
}

// Context is a block's marker, for its first slot: what its structure
// spells before its text.
func (product) Context(r doc.Reader, ref doc.SlotRef) any {
	if ref.Slot != 0 {
		return nil
	}
	n, ok := r.Node(ref.Node)
	if !ok {
		return nil
	}
	if m, ok := MarkerOf(r, n); ok {
		return m
	}
	return nil
}

// NodeRoles lists the roles a node kind provides. Every object with a
// slot of its own is Leavable: its text, however deep, is left beside it
// and returned to at its end, whatever the object.
func NodeRoles(k doc.Kind) []vocab.RoleID {
	roles := kindRoles(k)
	if spec, ok := Schema().Kind(k); ok && spec.Object && len(spec.Slots) > 0 {
		roles = append([]vocab.RoleID{vocab.RoleLeavable.ID}, roles...)
	}
	return roles
}

func kindRoles(k doc.Kind) []vocab.RoleID {
	switch k {
	case Heading:
		return []vocab.RoleID{vocab.RoleLeveled.ID}
	case Item:
		return []vocab.RoleID{vocab.RoleLeveled.ID, vocab.RoleContinued.ID}
	case Formula:
		return []vocab.RoleID{vocab.RoleCloser.ID, vocab.RoleStructure.ID}
	case Code:
		return []vocab.RoleID{vocab.RoleLeveled.ID, vocab.RoleStructure.ID}
	case Table:
		return []vocab.RoleID{vocab.RoleStructure.ID, vocab.RoleSpanned.ID}
	case Math, Figure, Divider:
		return []vocab.RoleID{vocab.RoleStructure.ID}
	}
	return nil
}

// NodeRole returns n's implementation of role.
func NodeRole(r doc.Reader, n *doc.Node, role vocab.RoleID) (any, bool) {
	switch role {
	case vocab.RoleLeveled.ID:
		switch n.Kind() {
		case Heading:
			return headingLevel{id: n.ID(), level: HeadingLevel(n)}, true
		case Item:
			return itemLevel(n.ID()), true
		case Code:
			return plainLevel(n.ID()), true
		}
	case vocab.RoleCloser.ID:
		if n.Kind() == Formula {
			return spelled(FormulaSpelling), true
		}
	case vocab.RoleContinued.ID:
		if n.Kind() == Item {
			return itemNext{task: TaskOf(n)}, true
		}
	case vocab.RoleLeavable.ID:
		if r.Schema().IsObject(n.Kind()) {
			after, ok1 := vocab.Beside(r, n.ID(), vocab.Forward)
			last, ok2 := vocab.LastTextSlot(n)
			if !ok1 || !ok2 {
				return nil, false
			}
			ln, _ := r.Node(last.Node)
			return leavable{
				exit:  doc.CaretAt(after),
				entry: doc.CaretAt(doc.At(last, ln.Slot(last.Slot).Len())),
			}, true
		}
	case vocab.RoleSpanned.ID:
		if n.Kind() == Table {
			return tableGrid{r: r, table: n.ID()}, true
		}
	case vocab.RoleStructure.ID:
		for _, provided := range NodeRoles(n.Kind()) {
			if provided == vocab.RoleStructure.ID {
				edge, ok := vocab.NodeEdge(r, n.ID())
				return structure{id: n.ID(), edge: edge}, ok
			}
		}
	}
	return nil, false
}

type headingLevel struct {
	id    doc.ID
	level int
}

// Step moves a heading one level, as one more "#" (Forward) or one fewer
// (Backward); below level 1 it becomes a paragraph, and 6 is the deepest.
func (h headingLevel) Step(dir vocab.Direction) vocab.Edit {
	next := h.level + int(dir)
	switch {
	case next < 1:
		return vocab.Ops(doc.SetNode{ID: h.id, Kind: Paragraph})
	case next > 6:
		next = 6
	}
	return vocab.Ops(doc.SetNode{ID: h.id, Kind: Heading, Attrs: doc.MakeAttrs(doc.Field{Num: FieldLevel, Value: doc.Int(int64(next))})})
}

// itemLevel is an item's nesting depth.
type itemLevel doc.ID

// Step nests the item one level deeper, into the item before it
// (Forward), or one level out (Backward): out of a nested list to follow
// the item holding it, and out of a top-level list as a paragraph.
func (i itemLevel) Step(dir vocab.Direction) vocab.Edit {
	id := doc.ID(i)
	if dir > 0 {
		return func(tx *doc.Tx) error {
			if _, _, index, ok := vocab.Index(tx, id); !ok || index == 0 {
				return nil
			}
			return vocab.Sink(id)(tx)
		}
	}
	return vocab.Outdent(id)
}

// spelled is a scope its spelling closes, as "$" closes a formula.
type spelled Spelling

func (s spelled) Closes(text string) bool { return text == string(s) }

// plainLevel is a block one level above a paragraph: stepping it down
// makes it one, keeping its text; nothing is above it.
type plainLevel doc.ID

func (p plainLevel) Step(dir vocab.Direction) vocab.Edit {
	if dir > 0 {
		return vocab.Sequence()
	}
	return vocab.Unmark(doc.ID(p))
}

// itemNext continues a list: an item after an item, an open task after a
// task.
type itemNext struct{ task Task }

func (i itemNext) Next() (doc.Kind, doc.Attrs) {
	if i.task != NoTask {
		return Item, doc.MakeAttrs(doc.Field{Num: FieldTask, Value: doc.Int(int64(TaskOpen))})
	}
	return Item, doc.Attrs{}
}

// tableGrid is a table's shape: a range between two of its cells selects the
// rectangle of cells between them.
type tableGrid struct {
	r     doc.Reader
	table doc.ID
}

func (g tableGrid) Span(anchor, head doc.Caret) (doc.Selection, bool) {
	a, ok1 := g.cellOf(anchor.Pos)
	h, ok2 := g.cellOf(head.Pos)
	if !ok1 || !ok2 {
		return doc.Selection{}, false
	}
	return doc.CellSelection(doc.CellRange{Table: g.table, Anchor: a, Head: h}), true
}

// Copy is a cell range's cells as a table of their own: the rows and
// columns it covers, their columns' shape, and the header when it holds
// the first row.
func (g tableGrid) Copy(sel doc.Selection) (vocab.Slice, bool) {
	cells := vocab.Cells(g.r, sel.Cells)
	t, ok := g.r.Node(g.table)
	if sel.Kind != doc.SelectCells || len(cells) == 0 || !ok {
		return vocab.Slice{}, false
	}
	_, first, _ := vocab.Locate(g.r, cells[0].ID())
	_, last, _ := vocab.Locate(g.r, cells[len(cells)-1].ID())
	shape := TableShapeOf(t)
	shape.Columns = shape.Columns[first.Column : last.Column+1]
	shape.Header = shape.Header && first.Row == 0
	width := last.Column - first.Column + 1
	var rows []*doc.Node
	for i := 0; i < len(cells); i += width {
		row := make([]*doc.Node, width)
		for j := range row {
			row[j] = vocab.Fresh(cells[i+j])
		}
		rows = append(rows, doc.NewNode(Row, doc.Attrs{}, doc.BlockSlot(row...)))
	}
	return vocab.Slice{Nodes: []*doc.Node{doc.NewNode(Table, shape.Attrs(), doc.BlockSlot(rows...))}}, true
}

// cellOf is where in the table the cell holding p stands.
func (g tableGrid) cellOf(p doc.Pos) (doc.CellCoord, bool) {
	for _, id := range append([]doc.ID{p.Node}, doc.Ancestors(g.r, p.Node)...) {
		if t, at, ok := vocab.Locate(g.r, id); ok && t.Table.ID() == g.table {
			return at, true
		}
	}
	return doc.CellCoord{}, false
}

type leavable struct{ exit, entry doc.Caret }

func (l leavable) Exit() doc.Caret  { return l.exit }
func (l leavable) Entry() doc.Caret { return l.entry }

// structure is a node as a whole: its place, removed with all it holds.
type structure struct {
	id   doc.ID
	edge vocab.Edge
}

func (s structure) Extent() []vocab.Edge { return []vocab.Edge{s.edge} }
func (s structure) Remove() vocab.Edit   { return vocab.RemoveNode(s.id) }

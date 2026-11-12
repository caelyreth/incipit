package vocab

import "github.com/caelyreth/neverland/editor/doc"

// A grid is an object whose first slot holds rows, each row's first slot
// its cells, every row as long as the others. The vocabulary reads and
// edits a grid by that shape alone: new rows and cells take the kinds and
// attributes of those beside them, so it never names a kind, and what a
// profile keeps about its columns it keeps in step itself.

// Grid is a grid object read at one version.
type Grid struct {
	Table *doc.Node
	Rows  []*doc.Node
}

// GridOf reads the grid object id, false when id is no grid.
func GridOf(r doc.Reader, id doc.ID) (Grid, bool) {
	t, ok := r.Node(id)
	if !ok {
		return Grid{}, false
	}
	return GridNode(r.Schema(), t)
}

// GridNode reads n as a grid, attached to a page or not, as a slice holds
// it.
func GridNode(s *doc.Schema, n *doc.Node) (Grid, bool) {
	if !s.IsObject(n.Kind()) || n.NumSlots() == 0 || n.Slot(0).IsInline() || n.Slot(0).Len() == 0 {
		return Grid{}, false
	}
	rows := n.Slot(0).Children()
	for _, row := range rows {
		if row.NumSlots() == 0 || row.Slot(0).IsInline() || row.Slot(0).Len() != rows[0].Slot(0).Len() {
			return Grid{}, false
		}
	}
	return Grid{Table: n, Rows: rows}, true
}

// Size is how many rows and columns the grid has.
func (g Grid) Size() (rows, columns int) { return len(g.Rows), g.Rows[0].Slot(0).Len() }

// Cell is the cell at c.
func (g Grid) Cell(c doc.CellCoord) *doc.Node { return g.Rows[c.Row].Slot(0).Child(c.Column) }

// Locate is the grid a cell stands in and where in it.
func Locate(r doc.Reader, cell doc.ID) (Grid, doc.CellCoord, bool) {
	row, _, ok := r.Parent(cell)
	if !ok {
		return Grid{}, doc.CellCoord{}, false
	}
	table, _, ok := r.Parent(row)
	if !ok {
		return Grid{}, doc.CellCoord{}, false
	}
	g, ok := GridOf(r, table)
	if !ok {
		return Grid{}, doc.CellCoord{}, false
	}
	for i, rn := range g.Rows {
		if rn.ID() == row {
			n, _ := r.Node(cell)
			j, _ := rn.Slot(0).IndexOf(n)
			return g, doc.CellCoord{Row: i, Column: j}, true
		}
	}
	return Grid{}, doc.CellCoord{}, false
}

// Cells are the cells a cell range covers, in reading order, clamped to
// its grid.
func Cells(r doc.Reader, cr doc.CellRange) []*doc.Node {
	g, ok := GridOf(r, cr.Table)
	if !ok {
		return nil
	}
	rows, columns := g.Size()
	r0, r1 := min(cr.Anchor.Row, cr.Head.Row), max(cr.Anchor.Row, cr.Head.Row)
	c0, c1 := min(cr.Anchor.Column, cr.Head.Column), max(cr.Anchor.Column, cr.Head.Column)
	if cr.WholeColumns {
		r0, r1 = 0, rows-1
	}
	if cr.WholeRows {
		c0, c1 = 0, columns-1
	}
	var out []*doc.Node
	for i := max(r0, 0); i <= min(r1, rows-1); i++ {
		for j := max(c0, 0); j <= min(c1, columns-1); j++ {
			out = append(out, g.Cell(doc.CellCoord{Row: i, Column: j}))
		}
	}
	return out
}

// blank is an empty copy of n: its kinds and attributes, without text.
func blank(n *doc.Node) *doc.Node {
	slots := make([]doc.Slot, n.NumSlots())
	for i, s := range n.Slots() {
		if s.IsInline() {
			slots[i] = doc.InlineSlot()
			continue
		}
		children := make([]*doc.Node, s.Len())
		for j, c := range s.Children() {
			children[j] = blank(c)
		}
		slots[i] = doc.BlockSlot(children...)
	}
	return doc.NewNode(n.Kind(), n.Attrs(), slots...)
}

// gridEdit reads the grid table at the edit's version.
func gridEdit(table doc.ID, edit func(tx *doc.Tx, g Grid) error) Edit {
	return func(tx *doc.Tx) error {
		g, ok := GridOf(tx, table)
		if !ok {
			return doc.ErrNoNode
		}
		return edit(tx, g)
	}
}

// InsertRows puts count empty rows before row at, at the row count after
// the last; each is a blank of the row it stands by.
func InsertRows(table doc.ID, at, count int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		if at < 0 || at > len(g.Rows) || count < 1 {
			return doc.ErrRange
		}
		like := g.Rows[min(at, len(g.Rows)-1)]
		rows := make([]*doc.Node, count)
		for i := range rows {
			rows[i] = blank(like)
		}
		return tx.Apply(doc.Insert{Parent: table, Slot: 0, Index: at, Nodes: rows})
	})
}

// InsertColumns puts count empty cells before column at in every row, at
// the column count after the last.
func InsertColumns(table doc.ID, at, count int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		_, columns := g.Size()
		if at < 0 || at > columns || count < 1 {
			return doc.ErrRange
		}
		for _, row := range g.Rows {
			like := row.Slot(0).Child(min(at, columns-1))
			cells := make([]*doc.Node, count)
			for i := range cells {
				cells[i] = blank(like)
			}
			if err := tx.Apply(doc.Insert{Parent: row.ID(), Slot: 0, Index: at, Nodes: cells}); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveRows takes rows from up to to away. A grid keeps a row: taking
// them all is the table's going, which is its owner's.
func RemoveRows(table doc.ID, from, to int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		if from < 0 || to > len(g.Rows) || from >= to || to-from == len(g.Rows) {
			return doc.ErrRange
		}
		ids := make([]doc.ID, 0, to-from)
		for _, row := range g.Rows[from:to] {
			ids = append(ids, row.ID())
		}
		return tx.Apply(doc.Remove{IDs: ids})
	})
}

// RemoveColumns takes columns from up to to away from every row. A grid
// keeps a column.
func RemoveColumns(table doc.ID, from, to int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		_, columns := g.Size()
		if from < 0 || to > columns || from >= to || to-from == columns {
			return doc.ErrRange
		}
		var ids []doc.ID
		for _, row := range g.Rows {
			for _, c := range row.Slot(0).Children()[from:to] {
				ids = append(ids, c.ID())
			}
		}
		return tx.Apply(doc.Remove{IDs: ids})
	})
}

// MoveRow moves row from to stand at to, the rows between making room.
func MoveRow(table doc.ID, from, to int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		if from < 0 || from >= len(g.Rows) || to < 0 || to >= len(g.Rows) {
			return doc.ErrRange
		}
		if from == to {
			return nil
		}
		return tx.Apply(doc.Move{ID: g.Rows[from].ID(), Parent: table, Slot: 0, Index: gapFor(from, to)})
	})
}

// MoveColumn moves column from to stand at to in every row.
func MoveColumn(table doc.ID, from, to int) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		_, columns := g.Size()
		if from < 0 || from >= columns || to < 0 || to >= columns {
			return doc.ErrRange
		}
		if from == to {
			return nil
		}
		for _, row := range g.Rows {
			if err := tx.Apply(doc.Move{ID: row.Slot(0).Child(from).ID(), Parent: row.ID(), Slot: 0, Index: gapFor(from, to)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// gapFor is the gap a node at from goes to so that it stands at to: a
// move's gap is counted before the node leaves.
func gapFor(from, to int) int {
	if to > from {
		return to + 1
	}
	return to
}

// ClearCells empties each cell's text; the cells stay.
func ClearCells(cells []doc.ID) Edit {
	return func(tx *doc.Tx) error {
		for _, id := range cells {
			n, ok := tx.Node(id)
			if !ok {
				return doc.ErrNoNode
			}
			ref, ok := FirstTextSlot(n)
			if !ok || n.Slot(ref.Slot).Len() == 0 {
				continue
			}
			if err := tx.Apply(doc.EditText{At: ref, From: 0, To: n.Slot(ref.Slot).Len()}); err != nil {
				return err
			}
		}
		return nil
	}
}

// FillCells writes the text of src's cells into table's cells from at on,
// row by row, each cell's text replacing what the cell held. The table
// must already reach as far as src does.
func FillCells(table doc.ID, at doc.CellCoord, src Grid) Edit {
	return gridEdit(table, func(tx *doc.Tx, g Grid) error {
		rows, cols := g.Size()
		srows, scols := src.Size()
		if at.Row < 0 || at.Column < 0 || at.Row+srows > rows || at.Column+scols > cols {
			return doc.ErrRange
		}
		for i := range srows {
			for j := range scols {
				from := src.Cell(doc.CellCoord{Row: i, Column: j})
				to := g.Cell(doc.CellCoord{Row: at.Row + i, Column: at.Column + j})
				fref, ok1 := FirstTextSlot(from)
				tref, ok2 := FirstTextSlot(to)
				if !ok1 || !ok2 {
					continue
				}
				in := from.Slot(fref.Slot)
				items := freshItems(in.Content().Slice(0, in.Len()))
				if err := tx.Apply(doc.EditText{At: tref, From: 0, To: to.Slot(tref.Slot).Len(), Items: items}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

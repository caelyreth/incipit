package surface

import (
	"fmt"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// A table sets the lines of its cells out in a grid: each cell is the run
// of lines its text takes in the flow, one after another, row by row, so
// the caret, typing and selection go through a table as through any text,
// and only where its lines stand differs.

// CellAlign is how a column's cells set their text.
type CellAlign uint8

const (
	CellStart CellAlign = iota
	CellCenter
	CellEnd
)

// TableCell is one cell: the block it is, and the range of its text in the
// flow, which starts a line and ends one.
type TableCell struct {
	Node  model.NodeID
	Range model.UTF16Range
}

// Table is a grid of cells, every row as long as the others.
type Table struct {
	Rows [][]TableCell
	// Header sets the first row apart, as the names of the columns.
	Header bool
	// Align and Widths are each column's: how it sets its text, and the
	// share of the page's column it takes, 0 to be as wide as its text
	// asks and share what the others leave.
	Align  []CellAlign
	Widths []float32
	// Room is what a view opens in and below the table as it shows what
	// makes rows and columns there.
	Room TableRoom
}

// TableRoom is room a view opens in and below a table, in DIPs, and how
// far each row and column has come in. Its zero value opens none.
type TableRoom struct {
	// Below is room under the table, for a new row's area.
	Below float32
	// RowRoom and ColumnRoom open before row Row and column Column, for
	// an area making one there.
	Row, Column         int
	RowRoom, ColumnRoom float32
	// RowShown and ColumnShown are each row's and column's share of its
	// size, coming in from 0; all 1 when nil.
	RowShown, ColumnShown []float32
	// Scroll is how far a table wider than the room it shows in is
	// scrolled across, and Pull how far an edge's pull draws it on past
	// that, elastic, as a scroll goes past its end.
	Scroll, Pull float32
	// RowsApart and ColumnsApart are room a drag opens before each row
	// and column, the last after them all, where what it carries would
	// land.
	RowsApart, ColumnsApart []float32
}

// apart is the room at index of rooms, none past them.
func apart(rooms []float32, index int) float32 {
	if index < len(rooms) {
		return max(0, rooms[index])
	}
	return 0
}

// RowApart and ColumnApart are the room a drag opens before row and
// column.
func (r TableRoom) RowApart(row int) float32       { return apart(r.RowsApart, row) }
func (r TableRoom) ColumnApart(column int) float32 { return apart(r.ColumnsApart, column) }

// RowShare and ColumnShare are row's and column's share of their size.
func (r TableRoom) RowShare(row int) float32 {
	if row < len(r.RowShown) {
		return max(0, min(1, r.RowShown[row]))
	}
	return 1
}

func (r TableRoom) ColumnShare(column int) float32 {
	if column < len(r.ColumnShown) {
		return max(0, min(1, r.ColumnShown[column]))
	}
	return 1
}

// Columns is how many cells each row has.
func (t Table) Columns() int {
	if len(t.Rows) == 0 {
		return 0
	}
	return len(t.Rows[0])
}

// Node is the table's first cell, which names it.
func (t Table) Node() model.NodeID {
	if len(t.Rows) == 0 || len(t.Rows[0]) == 0 {
		return 0
	}
	return t.Rows[0][0].Node
}

// Range is the flow the table's cells take, from its first cell's start to
// its last's end.
func (t Table) Range() model.UTF16Range {
	if t.Columns() == 0 {
		return model.UTF16Range{}
	}
	last := t.Rows[len(t.Rows)-1]
	return model.NewUTF16Range(t.Rows[0][0].Range.Start, last[len(last)-1].Range.End)
}

// CellAt is the cell whose text holds offset, its row and its column.
func (t Table) CellAt(offset model.UTF16Offset) (int, int, bool) {
	for row, cells := range t.Rows {
		for column, cell := range cells {
			if cell.Range.Start <= offset && offset <= cell.Range.End {
				return row, column, true
			}
		}
	}
	return 0, 0, false
}

// WithTables is the flow with the tables its lines are set out in, in
// order, each cell starting a line.
func (f *Flow) WithTables(tables []Table) (*Flow, error) {
	if err := validateTables(f, tables); err != nil {
		return nil, err
	}
	next := *f
	next.tables = slices.Clone(tables)
	return &next, nil
}

func validateTables(f *Flow, tables []Table) error {
	previous := model.UTF16Offset(-1)
	for _, table := range tables {
		columns := table.Columns()
		if columns == 0 || len(table.Align) != columns || len(table.Widths) != columns {
			return fmt.Errorf("a table must have cells and each column's shape")
		}
		for _, row := range table.Rows {
			if len(row) != columns {
				return fmt.Errorf("a table's rows must be as long as each other")
			}
			for _, cell := range row {
				if cell.Range.Start <= previous || cell.Range.End < cell.Range.Start ||
					cell.Range.End > f.UTF16Len() || cell.Range.Start > 0 && !f.lineStart(cell.Range.Start) {
					return fmt.Errorf("a cell must start a line, in order")
				}
				previous = cell.Range.End
			}
		}
	}
	return nil
}

// Tables are the tables the flow's lines are set out in, in order.
func (f *Flow) Tables() []Table { return slices.Clone(f.tables) }

// TableAt is the table whose cells hold offset.
func (f *Flow) TableAt(offset model.UTF16Offset) (Table, bool) {
	for _, table := range f.tables {
		if r := table.Range(); r.Start <= offset && offset <= r.End {
			return table, true
		}
	}
	return Table{}, false
}

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
	// Align and Fit are each column's: how it sets its text, and whether
	// it fits its text on one line rather than wrap it as the table has
	// room.
	Align []CellAlign
	Fit   []bool
}

func (t Table) clone() Table {
	t.Rows = slices.Clone(t.Rows)
	for row := range t.Rows {
		t.Rows[row] = slices.Clone(t.Rows[row])
	}
	t.Align = slices.Clone(t.Align)
	t.Fit = slices.Clone(t.Fit)
	return t
}

func cloneTables(tables []Table) []Table {
	cloned := slices.Clone(tables)
	for index := range cloned {
		cloned[index] = cloned[index].clone()
	}
	return cloned
}

func (t Table) equal(other Table) bool {
	if t.Header != other.Header || !slices.Equal(t.Align, other.Align) ||
		!slices.Equal(t.Fit, other.Fit) || len(t.Rows) != len(other.Rows) {
		return false
	}
	for row := range t.Rows {
		if !slices.Equal(t.Rows[row], other.Rows[row]) {
			return false
		}
	}
	return true
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
	next.tables = cloneTables(tables)
	return &next, nil
}

func validateTables(f *Flow, tables []Table) error {
	previous := model.UTF16Offset(-1)
	for _, table := range tables {
		columns := table.Columns()
		if columns == 0 || len(table.Align) != columns || len(table.Fit) != columns {
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
func (f *Flow) Tables() []Table { return cloneTables(f.tables) }

// TableAt is the table whose cells hold offset.
func (f *Flow) TableAt(offset model.UTF16Offset) (Table, bool) {
	for _, table := range f.tables {
		if r := table.Range(); r.Start <= offset && offset <= r.End {
			return table.clone(), true
		}
	}
	return Table{}, false
}

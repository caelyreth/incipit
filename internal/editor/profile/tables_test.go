package profile

import (
	"errors"
	"fmt"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// grid is a table of rows of texts, its columns aligned start, center,
// end, and so on, so a column's shape shows where it went.
func grid(rows ...[]string) *doc.Node {
	t := NewTable(len(rows), len(rows[0]))
	shape := TableShapeOf(t)
	for i := range shape.Columns {
		shape.Columns[i].Align = Align(i % 3)
	}
	rs := make([]*doc.Node, len(rows))
	for i, row := range rows {
		cells := make([]*doc.Node, len(row))
		for j, text := range row {
			cells[j] = vocab.NewBlock(Schema(), Cell, doc.Attrs{}, doc.TextItem(text, nil))
		}
		rs[i] = doc.NewNode(Row, doc.Attrs{}, doc.BlockSlot(cells...))
	}
	return doc.NewNode(Table, shape.Attrs(), doc.BlockSlot(rs...))
}

func aligns(n *doc.Node) string {
	var s string
	for _, c := range TableShapeOf(n).Columns {
		s += fmt.Sprint(int(c.Align))
	}
	return s
}

// Every grid edit at every edge leaves a valid table, its columns' shape
// moving with its cells.
func TestGridEditsKeepTheTable(t *testing.T) {
	const ab = "table(row(cell(a) cell(b) cell(c)) row(cell(d) cell(e) cell(f)))"
	for _, tc := range []struct {
		name   string
		edit   func(doc.ID) vocab.Edit
		want   string
		aligns string
	}{
		{"row first", func(id doc.ID) vocab.Edit { return vocab.InsertRows(id, 0, 1) }, "table(row(cell() cell() cell()) row(cell(a) cell(b) cell(c)) row(cell(d) cell(e) cell(f)))", "012"},
		{"rows last", func(id doc.ID) vocab.Edit { return vocab.InsertRows(id, 2, 2) }, "table(row(cell(a) cell(b) cell(c)) row(cell(d) cell(e) cell(f)) row(cell() cell() cell()) row(cell() cell() cell()))", "012"},
		{"column first", func(id doc.ID) vocab.Edit { return InsertColumns(id, 0, 1) }, "table(row(cell() cell(a) cell(b) cell(c)) row(cell() cell(d) cell(e) cell(f)))", "0012"},
		{"columns last", func(id doc.ID) vocab.Edit { return InsertColumns(id, 3, 2) }, "table(row(cell(a) cell(b) cell(c) cell() cell()) row(cell(d) cell(e) cell(f) cell() cell()))", "01222"},
		{"remove row", func(id doc.ID) vocab.Edit { return vocab.RemoveRows(id, 0, 1) }, "table(row(cell(d) cell(e) cell(f)))", "012"},
		{"remove columns", func(id doc.ID) vocab.Edit { return RemoveColumns(id, 1, 3) }, "table(row(cell(a)) row(cell(d)))", "0"},
		{"move row down", func(id doc.ID) vocab.Edit { return vocab.MoveRow(id, 0, 1) }, "table(row(cell(d) cell(e) cell(f)) row(cell(a) cell(b) cell(c)))", "012"},
		{"move row up", func(id doc.ID) vocab.Edit { return vocab.MoveRow(id, 1, 0) }, "table(row(cell(d) cell(e) cell(f)) row(cell(a) cell(b) cell(c)))", "012"},
		{"move column last", func(id doc.ID) vocab.Edit { return MoveColumn(id, 0, 2) }, "table(row(cell(b) cell(c) cell(a)) row(cell(e) cell(f) cell(d)))", "120"},
		{"move column first", func(id doc.ID) vocab.Edit { return MoveColumn(id, 2, 0) }, "table(row(cell(c) cell(a) cell(b)) row(cell(f) cell(d) cell(e)))", "201"},
		{"move in place", func(id doc.ID) vocab.Edit { return MoveColumn(id, 1, 1) }, ab, "012"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := page(t, grid([]string{"a", "b", "c"}, []string{"d", "e", "f"}))
			table := s.Root().Slot(0).Child(0)
			s = edit(t, s, tc.edit(table.ID()))
			table, _ = s.Node(table.ID())
			if got := outline(table); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			if got := aligns(table); got != tc.aligns {
				t.Errorf("columns aligned %s, want %s", got, tc.aligns)
			}
		})
	}
}

// A grid keeps a row and a column, and an edit out of its range is
// refused whole.
func TestGridEditsOutOfRangeAreRefused(t *testing.T) {
	for name, e := range map[string]func(doc.ID) vocab.Edit{
		"every row":     func(id doc.ID) vocab.Edit { return vocab.RemoveRows(id, 0, 1) },
		"every column":  func(id doc.ID) vocab.Edit { return RemoveColumns(id, 0, 1) },
		"row past":      func(id doc.ID) vocab.Edit { return vocab.InsertRows(id, 2, 1) },
		"column past":   func(id doc.ID) vocab.Edit { return InsertColumns(id, 2, 1) },
		"move past":     func(id doc.ID) vocab.Edit { return MoveColumn(id, 0, 1) },
		"no rows added": func(id doc.ID) vocab.Edit { return vocab.InsertRows(id, 0, 0) },
	} {
		s := page(t, grid([]string{"a"}))
		table := s.Root().Slot(0).Child(0)
		tx := s.Begin(doc.Origin{Class: doc.OriginCommand})
		if err := e(table.ID())(tx); !errors.Is(err, doc.ErrRange) {
			t.Errorf("%s: %v, want a range error", name, err)
		}
	}
}

// A cell is located by its row and column, and clearing cells empties
// their text and keeps them.
func TestCellsAreLocatedAndCleared(t *testing.T) {
	s := page(t, grid([]string{"a", "b"}, []string{"c", "d"}))
	table := s.Root().Slot(0).Child(0)
	g, _ := vocab.GridOf(s, table.ID())
	d := g.Cell(doc.CellCoord{Row: 1, Column: 1})
	if _, at, ok := vocab.Locate(s, d.ID()); !ok || at != (doc.CellCoord{Row: 1, Column: 1}) {
		t.Fatalf("d located at %v, %v", at, ok)
	}
	cells := vocab.Cells(s, doc.CellRange{Table: table.ID(), Anchor: doc.CellCoord{Column: 1}, Head: doc.CellCoord{Column: 1}, WholeColumns: true})
	s = edit(t, s, vocab.ClearCells([]doc.ID{cells[0].ID(), cells[1].ID()}))
	table, _ = s.Node(table.ID())
	if got, want := outline(table), "table(row(cell(a) cell()) row(cell(c) cell()))"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// A table checks its rows' cells, so taking a cell from one row alone is
// refused, though the table itself is not touched.
func TestARowShortOfItsColumnsIsRefused(t *testing.T) {
	s := page(t, grid([]string{"a", "b"}, []string{"c", "d"}))
	g, _ := vocab.GridOf(s, s.Root().Slot(0).Child(0).ID())
	tx := s.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Remove{IDs: []doc.ID{g.Cell(doc.CellCoord{Row: 1, Column: 1}).ID()}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Commit(); !errors.Is(err, doc.ErrInvalid) {
		t.Fatalf("commit: %v, want the table refused", err)
	}
}

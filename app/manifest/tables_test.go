package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// placeTable puts a two-by-two table of cells a, b, c, d on the page.
func (p *page) placeTable() *doc.Node {
	p.t.Helper()
	cell := func(t string) *doc.Node {
		return doc.NewNode(profile.Cell, doc.Attrs{}, doc.InlineSlot(doc.Item{Text: t}))
	}
	row := func(a, b string) *doc.Node {
		return doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(cell(a), cell(b)))
	}
	cols := doc.MakeAttrs(doc.Field{Num: profile.FieldColumns, Value: doc.List(doc.Attrs{}, doc.Attrs{})})
	p.do(vocab.Paste{Slice: vocab.Slice{Nodes: []*doc.Node{doc.NewNode(profile.Table, cols, doc.BlockSlot(row("a", "b"), row("c", "d")))}}})
	return p.e.Snapshot().Root().Slot(0).Child(0)
}

func cellAt(table *doc.Node, row, col int) *doc.Node {
	return table.Slot(0).Child(row).Slot(0).Child(col)
}

// A table's cells are its text: a caret stands in them, and deleting at a
// cell's edge neither merges cells nor lifts one out of its row.
func TestATablesCellsAreTextThatKeepsItsGrid(t *testing.T) {
	for _, tc := range []struct {
		row, col, at int
		dir          vocab.Direction
	}{
		{0, 1, 0, vocab.Backward}, {0, 0, 0, vocab.Backward}, {0, 0, 1, vocab.Forward}, {1, 0, 0, vocab.Backward},
	} {
		p := open(t)
		table := p.placeTable()
		cell := cellAt(table, tc.row, tc.col)
		p.do(vocab.Place{Stop: doc.CaretAt(doc.Pos{Node: cell.ID(), Offset: tc.at})})
		if h := p.selection().Head.Pos; h.Node != cell.ID() {
			t.Fatalf("caret at %v, want in cell %d,%d", h, tc.row, tc.col)
		}
		p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: tc.dir})
		p.want("page(table(row(cell(a) cell(b)) row(cell(c) cell(d))))")
	}
}

// A cell range spans its cells' text, whole, in reading order.
func TestACellRangeSpansItsCells(t *testing.T) {
	p := open(t)
	table := p.placeTable()
	r := p.e.Snapshot()
	for _, tc := range []struct {
		cr   doc.CellRange
		want []string
	}{
		{doc.CellRange{Table: table.ID(), Anchor: doc.CellCoord{Row: 1, Column: 1}, Head: doc.CellCoord{}}, []string{"a", "b", "c", "d"}},
		{doc.CellRange{Table: table.ID(), Anchor: doc.CellCoord{Column: 1}, Head: doc.CellCoord{Column: 1}, WholeColumns: true}, []string{"b", "d"}},
		{doc.CellRange{Table: table.ID(), Anchor: doc.CellCoord{Row: 1}, Head: doc.CellCoord{Row: 1}, WholeRows: true}, []string{"c", "d"}},
	} {
		pieces := vocab.SelectionExtent(r, doc.CellSelection(tc.cr))
		if len(pieces) != len(tc.want) {
			t.Fatalf("%+v spans %d pieces, want %d", tc.cr, len(pieces), len(tc.want))
		}
		for i, e := range pieces {
			c, _ := r.Node(e.From.Pos.Node)
			if c.Kind() != profile.Cell || e.To.Pos.Node != c.ID() || e.From.Offset != 0 || e.To.Offset != len(c.Text()) || c.Text() != tc.want[i] {
				t.Errorf("%+v piece %d is %v..%v, want the text of cell %q", tc.cr, i, e.From, e.To, tc.want[i])
			}
		}
	}
}

// inCell is where the caret stands in the table: the cell's row and column
// and the offset in its text.
func (p *page) inCell() (doc.CellCoord, int) {
	p.t.Helper()
	s := p.selection()
	if s.Kind != doc.SelectText || !s.IsCollapsed() {
		p.t.Fatalf("selection %+v, want a caret", s)
	}
	_, at, ok := vocab.Locate(p.e.Snapshot(), s.Head.Pos.Node)
	if !ok {
		p.t.Fatalf("caret at %v, not in a cell", s.Head.Pos)
	}
	return at, s.Head.Pos.Offset
}

func (p *page) wantCell(row, col, offset int) {
	p.t.Helper()
	if at, off := p.inCell(); at != (doc.CellCoord{Row: row, Column: col}) || off != offset {
		p.t.Fatalf("caret in cell %v at %d, want %d,%d at %d", at, off, row, col, offset)
	}
}

// A table inserted on an empty page takes its place and the caret; Tab
// and Shift+Tab walk its cells, Tab past the last making a row.
func TestTabWalksATablesCells(t *testing.T) {
	p := open(t)
	p.do(profile.InsertTable{Rows: 2, Columns: 2})
	p.want("page(table(row(cell() cell()) row(cell() cell())))")
	p.wantCell(0, 0, 0)
	for _, s := range []string{"a", "b", "c", "d"} {
		p.typeText(s)
		p.do(vocab.Leave{})
	}
	p.want("page(table(row(cell(a) cell(b)) row(cell(c) cell(d)) row(cell() cell())))")
	p.wantCell(2, 0, 0)
	p.do(vocab.Return{})
	p.wantCell(1, 1, 1)
	p.do(vocab.Return{})
	p.wantCell(1, 0, 1)
	p.do(vocab.Return{})
	p.wantCell(0, 1, 1)
	p.do(vocab.Undo{})
	p.want("page(table(row(cell(a) cell(b)) row(cell(c) cell(d))))")
}

// Tab in a formula in a cell leaves the formula before the cell.
func TestTabLeavesAFormulaInACellFirst(t *testing.T) {
	p := open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 2})
	p.typeText("$x")
	p.do(vocab.Leave{})
	if _, _, ok := vocab.Locate(p.e.Snapshot(), p.selection().Head.Pos.Node); !ok {
		t.Fatalf("caret at %v, want it out of the formula in its cell", p.selection().Head.Pos)
	}
	p.wantCell(0, 0, 1)
	p.do(vocab.Leave{})
	p.wantCell(0, 1, 0)
}

// Return goes down a column, making a row past the last; on an empty last
// row it leaves the table, the row going and writing going on below.
func TestReturnGoesDownAColumnAndLeavesFromAnEmptyRow(t *testing.T) {
	p := open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 2})
	p.do(vocab.Leave{})
	p.typeText("a")
	p.do(vocab.Break{})
	p.wantCell(1, 1, 0)
	p.want("page(table(row(cell() cell(a)) row(cell() cell())))")
	p.do(vocab.Break{})
	p.want("page(table(row(cell() cell(a))) paragraph())")
	p.typeText("b")
	p.want("page(table(row(cell() cell(a))) paragraph(b))")

	// A table of one empty row goes, the paragraph in its place.
	p = open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 2})
	p.do(vocab.Break{})
	p.want("page(paragraph())")

	// Shift+Return keeps lines in a cell.
	p = open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 1})
	p.typeText("a")
	p.do(vocab.SoftBreak{})
	p.typeText("b")
	p.want("page(table(row(cell(a\nb))))")
}

// Deleting at a cell's edge steps into the next cell; out of the table's
// text it holds the table whole, as any object, and takes it next.
func TestDeletingAtCellEdgesStepsAndHoldsTheTable(t *testing.T) {
	p := open(t)
	p.typeText("x")
	p.do(profile.InsertTable{Rows: 2, Columns: 2})
	p.want("page(paragraph(x) table(row(cell() cell()) row(cell() cell())))")
	for _, s := range []string{"a", "b", "c", "d"} {
		p.typeText(s)
		p.do(vocab.Leave{})
	}
	p.do(vocab.Undo{}) // the row Tab made
	const full = "page(paragraph(x) table(row(cell(a) cell(b)) row(cell(c) cell(d))))"
	p.want(full)
	p.do(vocab.Place{Stop: doc.CaretAt(doc.Pos{Node: cellAt(p.e.Snapshot().Root().Slot(0).Child(1), 1, 0).ID()})})
	p.do(backspace)
	p.wantCell(0, 1, 1)
	p.do(left)
	p.do(backspace)
	p.wantCell(0, 0, 1)
	p.do(left)
	p.do(backspace)
	p.want(full)
	if p.armed() == 0 {
		t.Fatal("Backspace out of the first cell does not hold the table")
	}
	p.do(backspace)
	p.want("page(paragraph(x))")

	p = open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 1})
	p.typeText("a")
	p.do(forward)
	if p.armed() == 0 {
		t.Fatal("Delete out of the last cell does not hold the table")
	}
}

// The table commands edit the lines the caret stands in, the caret
// staying in its column or row; removing every line takes the table.
func TestTableCommandsEditTheCaretsLines(t *testing.T) {
	p := open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 2})
	p.typeText("a")
	p.do(vocab.Leave{})
	p.typeText("b")
	steps := []struct {
		in       vocab.Intent
		want     string
		row, col int
	}{
		{profile.EditGrid{Op: profile.GridInsert, Axis: profile.Rows}, "page(table(row(cell(a) cell(b)) row(cell() cell())))", 1, 1},
		{profile.EditGrid{Op: profile.GridInsert, Axis: profile.Rows, Before: true}, "page(table(row(cell(a) cell(b)) row(cell() cell()) row(cell() cell())))", 1, 1},
		{profile.EditGrid{Op: profile.GridRemove, Axis: profile.Rows}, "page(table(row(cell(a) cell(b)) row(cell() cell())))", 1, 1},
		{profile.EditGrid{Op: profile.GridInsert, Axis: profile.Columns, Before: true}, "page(table(row(cell(a) cell() cell(b)) row(cell() cell() cell())))", 1, 1},
		{profile.EditGrid{Op: profile.GridInsert, Axis: profile.Columns}, "page(table(row(cell(a) cell() cell() cell(b)) row(cell() cell() cell() cell())))", 1, 2},
		{profile.EditGrid{Op: profile.GridRemove, Axis: profile.Columns}, "page(table(row(cell(a) cell() cell(b)) row(cell() cell() cell())))", 1, 2},
		{profile.EditGrid{Op: profile.GridMove, Axis: profile.Columns, Table: 0, To: 0}, "page(table(row(cell(b) cell(a) cell()) row(cell() cell() cell())))", 1, 0},
	}
	for _, s := range steps {
		p.do(s.in)
		p.want(s.want)
		p.wantCell(s.row, s.col, 0)
	}
	for range steps {
		p.do(vocab.Undo{})
	}
	p.want("page(table(row(cell(a) cell(b))))")
	for range steps {
		p.do(vocab.Redo{})
	}
	p.want(steps[len(steps)-1].want)

	p.do(profile.EditGrid{Op: profile.GridRemove, Axis: profile.Rows})
	p.do(profile.EditGrid{Op: profile.GridRemove, Axis: profile.Rows})
	p.want("page(paragraph())")
}

// A resize grows by empty lines and shrinks only by empty ones; a
// column's shape, the header and alignment are the table's.
func TestATableResizesAndShapesItsColumns(t *testing.T) {
	p := open(t)
	p.do(profile.InsertTable{Rows: 1, Columns: 1})
	p.typeText("a")
	table := p.e.Snapshot().Root().Slot(0).Child(0).ID()
	p.do(profile.ResizeTable{Table: table, Rows: 2, Columns: 3})
	p.want("page(table(row(cell(a) cell() cell()) row(cell() cell() cell())))")
	p.do(profile.ResizeTable{Table: table, Rows: 1, Columns: 1})
	p.want("page(table(row(cell(a))))")

	p.do(profile.AlignColumns{To: profile.AlignEnd})
	p.do(profile.ToggleHeader{})
	p.do(profile.ShapeColumn{Table: table, Column: 0, Shape: profile.Column{Align: profile.AlignEnd, Width: 120, Fit: true}})
	n, _ := p.e.Snapshot().Node(table)
	if s := profile.TableShapeOf(n); s.Header || s.Columns[0] != (profile.Column{Align: profile.AlignEnd, Width: 120, Fit: true}) {
		t.Fatalf("shape %+v", s)
	}
	p.do(profile.DeleteTable{})
	p.want("page(paragraph())")
}

// Return after a row written between pipes makes it a table's header, the
// caret in the row below; with Markdown tables off it is text.
func TestAPipeRowBecomesATable(t *testing.T) {
	p := open(t)
	p.typeText(`| Tide | Hei\|ght |`)
	p.do(vocab.Break{})
	p.want("page(table(row(cell(Tide) cell(Hei|ght)) row(cell() cell())))")
	p.wantCell(1, 0, 0)

	p = openWith(t, policy.Values{"markdown.tables": false})
	p.typeText("| a |")
	p.do(vocab.Break{})
	p.want("page(paragraph(| a |) paragraph())")
}

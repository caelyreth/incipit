package editor

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// tableTexts is the page's first table, its cells' texts row by row.
func tableTexts(t *testing.T, session *Session) [][]string {
	t.Helper()
	document := session.Runtime().Snapshot().Document()
	tables := readTables(document)
	if len(tables) == 0 {
		t.Fatal("the page has no table")
	}
	rows := make([][]string, len(tables[0].Rows))
	for r, row := range tables[0].Rows {
		for _, id := range row {
			node, _ := document.Node(id)
			rows[r] = append(rows[r], node.Text().String())
		}
	}
	return rows
}

func expectTable(t *testing.T, session *Session, want ...[]string) {
	t.Helper()
	got := tableTexts(t, session)
	if !slices.EqualFunc(got, want, slices.Equal[[]string]) {
		t.Fatalf("table = %q, want %q", got, want)
	}
}

func caretCell(t *testing.T, session *Session) (int, int) {
	t.Helper()
	table, ok := TableOf(session.Runtime().Snapshot().Document(), session.Selection().Head().Node())
	if !ok {
		t.Fatal("the caret is not in a table")
	}
	row, column, _ := table.Find(session.Selection().Head().Node())
	return row, column
}

// tablePage is a page written as a table's row and Return: its header,
// and an empty row the caret is in.
func tablePage(t *testing.T) *Session {
	t.Helper()
	session, _ := newSessionFixture(t, "", false)
	typeSession(t, session, "| Name | Date |")
	run(t, session, TextNewline)
	return session
}

func tab(t *testing.T, session *Session) {
	t.Helper()
	if _, err := session.Tab(time.Now()); err != nil {
		t.Fatal(err)
	}
}

// A paragraph written as a row and Return is a table's header, the caret
// in the empty row below it; the page goes on below the table.
func TestARowAndReturnStartATable(t *testing.T) {
	session := tablePage(t)
	expectTable(t, session, []string{"Name", "Date"}, []string{"", ""})
	expectKinds(t, session, CellNode, CellNode, CellNode, CellNode, ParagraphNode)
	if row, column := caretCell(t, session); row != 1 || column != 0 {
		t.Fatalf("caret in %d,%d", row, column)
	}
	table, _ := TableOf(session.Runtime().Snapshot().Document(), session.Selection().Head().Node())
	if !table.Header {
		t.Fatal("the first row is not the header")
	}
}

// Tab goes cell by cell, the last making a row; Return goes down, and on
// an empty last row leaves the table, the row gone.
func TestTabAndReturnGoThroughTheTable(t *testing.T) {
	session := tablePage(t)
	typeSession(t, session, "Ada")
	tab(t, session)
	typeSession(t, session, "1815")
	tab(t, session)
	if row, column := caretCell(t, session); row != 2 || column != 0 {
		t.Fatalf("Tab on the last cell went to %d,%d", row, column)
	}
	typeSession(t, session, "Grace")
	if _, err := session.ExecuteTable(TablePreviousCell); err != nil {
		t.Fatal(err)
	}
	if row, column := caretCell(t, session); row != 1 || column != 1 {
		t.Fatalf("Shift-Tab went to %d,%d", row, column)
	}
	run(t, session, TextNewline)
	if row, column := caretCell(t, session); row != 2 || column != 1 {
		t.Fatalf("Return went to %d,%d", row, column)
	}
	run(t, session, TextNewline)
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", "1815"}, []string{"Grace", ""}, []string{"", ""})
	run(t, session, TextNewline)
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", "1815"}, []string{"Grace", ""})
	if node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Head().Node()); node.Kind() != ParagraphNode {
		t.Fatal("Return on an empty last row did not leave the table")
	}
}

// Rows and columns go in and out where asked, every row as long as the
// others, the first row keeping the columns' shape; Undo takes each back.
func TestRowsAndColumnsComeAndGo(t *testing.T) {
	session := tablePage(t)
	typeSession(t, session, "Ada")
	cell := session.Selection().Head().Node()
	if _, err := session.ShapeTable(cell, func(table *Table) { table.Widths[1] = 0.3 }); err != nil {
		t.Fatal(err)
	}
	if _, err := session.InsertTableRows(cell, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := session.InsertTableColumns(cell, 1, 1); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"", "", ""}, []string{"Name", "", "Date"}, []string{"Ada", "", ""})
	table, _ := TableOf(session.Runtime().Snapshot().Document(), cell)
	if table.Widths[2] != 0.3 || table.Widths[1] != 0 {
		t.Fatalf("widths = %v", table.Widths)
	}
	if _, err := session.DeleteTableColumns(cell, 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := session.DeleteTableRows(cell, 0, 0); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", ""})
	if err := session.SelectCell(cell); err != nil {
		t.Fatal(err)
	}
	if _, err := session.MoveTableRow(cell, 1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := session.MoveTableColumn(cell, 0, 1); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"", "Ada"}, []string{"Date", "Name"})
	if node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Head().Node()); node.Text().String() != "Ada" {
		t.Fatalf("the caret did not move with its cell: it is in %q", node.Text().String())
	}
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"Ada", ""}, []string{"Name", "Date"})
}

// Backspace and Delete at a cell's edge step to the cell beside it, never
// joining them; in an empty table Backspace selects it, and again takes it.
func TestCellsNeverJoin(t *testing.T) {
	session := tablePage(t)
	run(t, session, TextBackspace)
	if row, column := caretCell(t, session); row != 0 || column != 1 {
		t.Fatalf("Backspace at a cell's start went to %d,%d", row, column)
	}
	run(t, session, TextDelete)
	if row, column := caretCell(t, session); row != 1 || column != 0 {
		t.Fatalf("Delete at a cell's end went to %d,%d", row, column)
	}
	expectTable(t, session, []string{"Name", "Date"}, []string{"", ""})

	session, _ = newSessionFixture(t, "", false)
	if _, err := session.ExecuteTable(TableInsert); err != nil {
		t.Fatal(err)
	}
	run(t, session, TextBackspace)
	if selected, _ := session.VisualSelection(); selected.Empty() {
		t.Fatal("Backspace in an empty table did not select it")
	}
	run(t, session, TextBackspace)
	expectKinds(t, session, ParagraphNode, ParagraphNode)
}

// A selection across cells empties them; one across the whole table and
// past it takes the table.
func TestDeletingAcrossCells(t *testing.T) {
	session, _ := newSessionFixture(t, "before", false)
	at(t, session, 6)
	if _, err := session.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	text := session.Projection().Text()
	// From inside "a" to inside "d": the cells between empty, none go.
	if err := session.SelectProjected(model.UTF16Offset(len("before\n")), model.UTF16Offset(len(text)-2)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReplaceSelection("", kernel.EventOriginSemantic); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"", ""}, []string{"", "d"})
	if err := session.SelectProjected(3, model.UTF16Offset(session.Projection().UTF16Len())); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReplaceSelection("", kernel.EventOriginSemantic); err != nil {
		t.Fatal(err)
	}
	if len(readTables(session.Runtime().Snapshot().Document())) != 0 || pageTexts(session)[0] != "bef" {
		t.Fatalf("the table stayed: %q", pageTexts(session))
	}
}

// What is copied of a table is its cells between tabs and its rows
// between lines; text copied so reads back as a table.
func TestATableCopiesAsASpreadsheetReadsIt(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	copied, err := session.ReadableText(model.NewUTF16Range(0, session.Projection().UTF16Len()))
	if err != nil {
		t.Fatal(err)
	}
	if copied != "a\tb\nc\td\n" {
		t.Fatalf("copied %q", copied)
	}
	rows, ok := ParseTable(copied)
	if !ok || !slices.EqualFunc(rows, [][]string{{"a", "b"}, {"c", "d"}}, slices.Equal[[]string]) {
		t.Fatalf("read back %q", rows)
	}
	rows, ok = ParseTable("| x | y |\n|---|---|\n| 1 | 2 |")
	if !ok || len(rows) != 2 || rows[1][1] != "2" {
		t.Fatalf("a Markdown table reads %q", rows)
	}
	if _, ok := ParseTable("just a line"); ok {
		t.Fatal("a line read as a table")
	}
}

// Pasting a table into one fills it from the caret, growing it.
func TestFillingATableGrowsIt(t *testing.T) {
	session := tablePage(t)
	cell := session.Selection().Head().Node()
	if _, err := session.FillTable(cell, [][]string{{"1", "2", "3"}, {"4", "5", "6"}}); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"Name", "Date", ""}, []string{"1", "2", "3"}, []string{"4", "5", "6"})
	if _, err := session.ResizeTable(cell, 2, 2); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"Name", "Date", ""}, []string{"1", "2", "3"}, []string{"4", "5", "6"})
	if _, err := session.ResizeTable(cell, 4, 4); err != nil {
		t.Fatal(err)
	}
	if got := tableTexts(t, session); len(got) != 4 || len(got[0]) != 4 {
		t.Fatalf("resized to %d by %d", len(got), len(got[0]))
	}
}

// The flow sets each table out with its cells' ranges, each starting a
// line.
func TestTheFlowKnowsItsTables(t *testing.T) {
	session := tablePage(t)
	flow, err := session.SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	tables := flow.Tables()
	if len(tables) != 1 || tables[0].Columns() != 2 || len(tables[0].Rows) != 2 || !tables[0].Header {
		t.Fatalf("tables = %+v", tables)
	}
	if got := tables[0].Rows[0][1].Range; got != model.NewUTF16Range(5, 9) {
		t.Fatalf("Date's range = %v", got)
	}
}

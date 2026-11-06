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
	run(t, session, textNewline)
	return session
}

func tab(t *testing.T, session *Session) {
	t.Helper()
	if _, err := session.pressLeave(time.Now()); err != nil {
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
	if _, err := session.Do(Return, time.Now()); err != nil {
		t.Fatal(err)
	}
	if row, column := caretCell(t, session); row != 1 || column != 1 {
		t.Fatalf("Shift-Tab went to %d,%d", row, column)
	}
	run(t, session, textNewline)
	if row, column := caretCell(t, session); row != 2 || column != 1 {
		t.Fatalf("Return went to %d,%d", row, column)
	}
	run(t, session, textNewline)
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", "1815"}, []string{"Grace", ""}, []string{"", ""})
	run(t, session, textNewline)
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", "1815"}, []string{"Grace", ""})
	if node, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Head().Node()); node.Kind() != ParagraphNode {
		t.Fatal("Return on an empty last row did not leave the table")
	}
}

// Return in a written last row, from any of its cells, makes a row below
// and writing goes on in it, under the cell it came from.
func TestReturnInTheLastRowGrowsTheTable(t *testing.T) {
	session := tablePage(t)
	typeSession(t, session, "Ada")
	run(t, session, textNewline)
	if row, column := caretCell(t, session); row != 2 || column != 0 {
		t.Fatalf("Return went to %d,%d", row, column)
	}
	typeSession(t, session, "Grace")
	expectTable(t, session, []string{"Name", "Date"}, []string{"Ada", ""}, []string{"Grace", ""})
}

// Rows and columns go in and out where asked, every row as long as the
// others, the first row keeping the columns' shape; Undo takes each back.
func TestRowsAndColumnsComeAndGo(t *testing.T) {
	session := tablePage(t)
	typeSession(t, session, "Ada")
	cell := session.Selection().Head().Node()
	if _, err := session.ShapeTable(cell, func(table *Table) { table.Fit[1] = true }); err != nil {
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
	if !table.Fit[2] || table.Fit[1] {
		t.Fatalf("fit = %v", table.Fit)
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
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"Ada", ""}, []string{"Name", "Date"})
}

// Backspace and Delete at a cell's edge step to the cell beside it, never
// joining them; in an empty table Backspace selects it, and again takes it.
func TestCellsNeverJoin(t *testing.T) {
	session := tablePage(t)
	run(t, session, textBackspace)
	if row, column := caretCell(t, session); row != 0 || column != 1 {
		t.Fatalf("Backspace at a cell's start went to %d,%d", row, column)
	}
	run(t, session, textDelete)
	if row, column := caretCell(t, session); row != 1 || column != 0 {
		t.Fatalf("Delete at a cell's end went to %d,%d", row, column)
	}
	expectTable(t, session, []string{"Name", "Date"}, []string{"", ""})

	session, _ = newSessionFixture(t, "", false)
	if _, err := session.doTable(TableInsert); err != nil {
		t.Fatal(err)
	}
	run(t, session, textBackspace)
	if span := armedSpan(t, session); span.Empty() {
		t.Fatal("Backspace in an empty table did not arm it")
	}
	run(t, session, textBackspace)
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
	if _, err := session.replaceSelection("", kernel.EventOriginSemantic); err != nil {
		t.Fatal(err)
	}
	expectTable(t, session, []string{"", ""}, []string{"", "d"})
	if err := session.SelectProjected(3, model.UTF16Offset(session.Projection().UTF16Len())); err != nil {
		t.Fatal(err)
	}
	if _, err := session.replaceSelection("", kernel.EventOriginSemantic); err != nil {
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

// A block moves whole between two others, a table with all its cells, and
// never between a table's cells; Undo takes it back.
func TestBlocksMoveWhole(t *testing.T) {
	session, _ := newSessionFixture(t, "one", false)
	at(t, session, 3)
	if _, err := session.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	units := session.Units()
	if len(units) != 3 || !units[1].Table || units[2].Table {
		t.Fatalf("units = %+v", units)
	}
	one, table, after := units[0].Node, units[1].Node, units[2].Node
	if _, err := session.MoveBlock(one, 0); err != nil {
		t.Fatal(err)
	}
	if units = session.Units(); units[0].Node != table || units[2].Node != one {
		t.Fatalf("moved down: %+v", units)
	}
	if _, err := session.MoveBlock(table, one); err != nil {
		t.Fatal(err)
	}
	if units = session.Units(); units[0].Node != after || units[1].Node != table {
		t.Fatalf("the table moved to %+v", units)
	}
	expectTable(t, session, []string{"a", "b"}, []string{"c", "d"})
	cell := readTables(session.Runtime().Snapshot().Document())[0].Rows[1][0]
	if outcome, _ := session.MoveBlock(one, cell); outcome == kernel.OutcomeHandled {
		t.Fatal("a block moved between a table's cells")
	}
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	if units = session.Units(); units[0].Node != table {
		t.Fatalf("undo left %+v", units)
	}
}

// A selection of whole blocks, a table's or a picture's as much as a
// paragraph's, says so in its frame, its caret not standing in the block
// past it; one ending at a block's start, or within text, does not.
func TestASelectionOfWholeBlocksSaysSo(t *testing.T) {
	session, _ := newSessionFixture(t, "Alpha\n\nBravo", false)
	at(t, session, 12)
	if _, err := session.InsertTable([][]string{{"a", ""}, {"", ""}}); err != nil {
		t.Fatal(err)
	}
	at(t, session, 0)
	if _, err := session.InsertImage(sunset); err != nil {
		t.Fatal(err)
	}
	document := session.Runtime().Snapshot().Document()
	table := readTables(document)[0]
	picture, _ := session.BlockRange(DocumentBlocks(document)[1].ID())
	whole := func(name string, start, end model.UTF16Offset, want bool) {
		t.Helper()
		if err := session.SelectProjected(start, end); err != nil {
			t.Fatal(err)
		}
		frame, err := session.SurfaceFrame(time.Now(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if frame.Whole() != want {
			t.Fatalf("%s %d..%d whole = %v", name, start, end, frame.Whole())
		}
	}
	if err := session.SelectTable(table.First()); err != nil {
		t.Fatal(err)
	}
	selected, _ := session.VisualSelection()
	whole("the table", selected.Start, selected.End, true)
	whole("the picture", picture.Start, picture.End, true)
	whole("the page", 0, model.UTF16Offset(session.Projection().UTF16Len()), true)
	whole("a word", 1, 3, false)
	empty, _ := session.BlockRange(DocumentBlocks(document)[2].ID())
	bravo, _ := session.BlockRange(DocumentBlocks(document)[3].ID())
	whole("to the next block's start", empty.Start, bravo.Start, false)
	whole("a caret", 2, 2, false)
}

// A table's cell is a place like any other: Tab in a formula inside a cell
// leaves the formula first, and only then goes on to the next cell.
func TestTabLeavesAFormulaInACellBeforeTheCell(t *testing.T) {
	session := tablePage(t)
	typeKeys(t, session, "$x")
	if places := session.innerPlaces(); len(places) == 0 || places[0].Kind != PlaceFormula {
		t.Fatalf("typing $x in a cell put the caret in %#v, not a formula", places)
	}
	tab(t, session)
	if row, column := caretCell(t, session); row != 1 || column != 0 {
		t.Fatalf("leaving the formula went to cell %d,%d", row, column)
	}
	if places := session.innerPlaces(); len(places) != 0 {
		t.Fatalf("after leaving the formula, places = %#v", places)
	}
	tab(t, session)
	if row, column := caretCell(t, session); row != 1 || column != 1 {
		t.Fatalf("the second Tab went to cell %d,%d, not the next", row, column)
	}
}

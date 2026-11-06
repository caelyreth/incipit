package editor

import (
	"slices"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// TableOp is one of the operations a table answers at the caret.
type TableOp uint8

const (
	TableInsert TableOp = iota + 1
	TableRowAbove
	TableRowBelow
	TableColumnLeft
	TableColumnRight
	TableDeleteRow
	TableDeleteColumn
	TableDelete
	TableHeader
)

// doTable runs a table operation on the table the caret is in; Insert
// Table makes one where the caret is.
func (s *Session) doTable(command TableOp) (kernel.Outcome, error) {
	if command == TableInsert {
		return s.InsertTable(emptyRows(newTableRows, newTableColumns))
	}
	cell := s.presentation.Selection.Head().Node()
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return kernel.OutcomePass, nil
	}
	row, column, _ := table.Find(cell)
	switch command {
	case TableRowAbove:
		return s.InsertTableRows(cell, row, 1)
	case TableRowBelow:
		return s.InsertTableRows(cell, row+1, 1)
	case TableColumnLeft:
		return s.InsertTableColumns(cell, column, 1)
	case TableColumnRight:
		return s.InsertTableColumns(cell, column+1, 1)
	case TableDeleteRow:
		return s.DeleteTableRows(cell, row, row)
	case TableDeleteColumn:
		return s.DeleteTableColumns(cell, column, column)
	case TableDelete:
		return s.DeleteTable(cell)
	case TableHeader:
		return s.ShapeTable(cell, func(table *Table) { table.Header = !table.Header })
	}
	return kernel.OutcomePass, nil
}

// InTable reports whether the caret's block is a table cell.
func (s *Session) InTable() bool {
	if s == nil {
		return false
	}
	_, ok := TableOf(s.runtime.Snapshot().Document(), s.presentation.Selection.Head().Node())
	return ok
}

// InsertTable puts a table of rows on the page, its first row its header:
// in place of the empty paragraph the caret is in, or after the caret's
// block, with a paragraph after it to go on writing in when it ends the
// page. The caret goes to its first cell.
func (s *Session) InsertTable(rows [][]string) (kernel.Outcome, error) {
	at := s.presentation.Selection.Node()
	var land model.NodeID
	outcome, err := s.dispatchWriting(tableTrigger, "table", func(document *model.DocumentDraft) (bool, error) {
		current, ok := document.Node(at)
		if !ok || len(rows) == 0 || len(rows[0]) == 0 {
			return false, nil
		}
		index, reuse := blockIndex(document, at), at
		if !kindOf(current).Plain || blockLength(current) > 0 {
			index, reuse = index+1, 0
		}
		table, err := makeTable(document, index, rows, true, reuse)
		if err != nil {
			return false, err
		}
		land = table.First()
		return true, (&EditorInvoker{}).writingRoom(document, table)
	})
	return s.landIn(outcome, err, land, false)
}

// InsertTableRows makes count empty rows before row at of cell's table,
// at the row count after its last. The caret goes to the first new row,
// in its column.
func (s *Session) InsertTableRows(cell model.NodeID, at, count int) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		at = max(0, min(at, len(table.Rows)))
		column := s.caretColumn(table)
		grown, err := insertRows(document, table, at, max(1, count))
		if err != nil {
			return 0, err
		}
		return grown.Rows[at][column], nil
	})
}

// InsertTableColumns makes count empty columns before column at. The
// caret goes to the first new column, in its row.
func (s *Session) InsertTableColumns(cell model.NodeID, at, count int) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		at = max(0, min(at, table.Columns()))
		row := s.caretRow(table)
		grown, err := insertColumns(document, table, at, max(1, count))
		if err != nil {
			return 0, err
		}
		return grown.Rows[row][at], nil
	})
}

// DeleteTableRows takes the rows from first to last away; taking them all
// takes the table.
func (s *Session) DeleteTableRows(cell model.NodeID, first, last int) (kernel.Outcome, error) {
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return kernel.OutcomePass, nil
	}
	first, last = max(0, min(first, last)), min(len(table.Rows)-1, max(first, last))
	if first == 0 && last == len(table.Rows)-1 {
		return s.DeleteTable(cell)
	}
	// The caret goes on in its column, below the rows or above them, when
	// it was in one of them.
	survivor := model.NodeID(0)
	if row, column, in := table.Find(s.presentation.Selection.Head().Node()); in && row >= first && row <= last {
		survivor = table.Rows[min(last+1, len(table.Rows)-1)][column]
		if last+1 >= len(table.Rows) {
			survivor = table.Rows[first-1][column]
		}
	}
	return s.editTable(cell, survivor, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		_, err := removeRows(document, table, first, last)
		return 0, err
	})
}

// DeleteTableColumns takes the columns from first to last away; taking
// them all takes the table.
func (s *Session) DeleteTableColumns(cell model.NodeID, first, last int) (kernel.Outcome, error) {
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return kernel.OutcomePass, nil
	}
	first, last = max(0, min(first, last)), min(table.Columns()-1, max(first, last))
	if first == 0 && last == table.Columns()-1 {
		return s.DeleteTable(cell)
	}
	survivor := model.NodeID(0)
	if row, column, in := table.Find(s.presentation.Selection.Head().Node()); in && column >= first && column <= last {
		survivor = table.Rows[row][min(last+1, table.Columns()-1)]
		if last+1 >= table.Columns() {
			survivor = table.Rows[row][first-1]
		}
	}
	return s.editTable(cell, survivor, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		_, err := removeColumns(document, table, first, last)
		return 0, err
	})
}

// DeleteTable takes cell's table away, an empty paragraph left where it
// was, the caret in it.
func (s *Session) DeleteTable(cell model.NodeID) (kernel.Outcome, error) {
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return kernel.OutcomePass, nil
	}
	return s.editTable(cell, table.First(), func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		return table.First(), removeTable(document, table)
	})
}

// MoveTableRow moves row from of cell's table to stand at to, the caret
// keeping its place in the text.
func (s *Session) MoveTableRow(cell model.NodeID, from, to int) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		if from < 0 || from >= len(table.Rows) || to < 0 || to >= len(table.Rows) || from == to {
			return 0, errNoChange
		}
		_, err := moveRow(document, table, from, to)
		return 0, err
	})
}

// MoveTableColumn moves column from to stand at to.
func (s *Session) MoveTableColumn(cell model.NodeID, from, to int) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		if from < 0 || from >= table.Columns() || to < 0 || to >= table.Columns() || from == to {
			return 0, errNoChange
		}
		_, err := moveColumn(document, table, from, to)
		return 0, err
	})
}

// ResizeTable makes cell's table rows by columns: it grows by empty rows
// and columns at its end, and shrinks by its last ones only while they
// hold nothing, so no text goes with a drag.
func (s *Session) ResizeTable(cell model.NodeID, rows, columns int) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		rows, columns = max(1, rows), max(1, columns)
		start := table
		var err error
		for len(table.Rows) > rows && emptyRow(document, table.Rows[len(table.Rows)-1]) &&
			!slices.Contains(table.Rows[len(table.Rows)-1], s.presentation.Selection.Head().Node()) {
			if table, err = removeRows(document, table, len(table.Rows)-1, len(table.Rows)-1); err != nil {
				return 0, err
			}
		}
		for table.Columns() > columns && emptyColumn(document, table, table.Columns()-1) &&
			!columnHolds(table, table.Columns()-1, s.presentation.Selection.Head().Node()) {
			if table, err = removeColumns(document, table, table.Columns()-1, table.Columns()-1); err != nil {
				return 0, err
			}
		}
		if more := rows - len(table.Rows); more > 0 {
			if table, err = insertRows(document, table, len(table.Rows), more); err != nil {
				return 0, err
			}
		}
		if more := columns - table.Columns(); more > 0 {
			if table, err = insertColumns(document, table, table.Columns(), more); err != nil {
				return 0, err
			}
		}
		if len(table.Rows) == len(start.Rows) && table.Columns() == start.Columns() {
			return 0, errNoChange
		}
		return 0, nil
	})
}

// ShapeTable changes cell's table's header and columns as change has it:
// change sees the table and may set Header, Align and Fit.
func (s *Session) ShapeTable(cell model.NodeID, change func(*Table)) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		next := table.clone()
		change(&next)
		next.Rows = table.Rows
		if next.Header == table.Header && slices.Equal(next.Align, table.Align) && slices.Equal(next.Fit, table.Fit) {
			return 0, errNoChange
		}
		return 0, writeTable(document, next)
	})
}

// ClearTableCells empties cells, as Delete does a selection of whole
// cells; the cells stay.
func (s *Session) ClearTableCells(cells []model.NodeID) (kernel.Outcome, error) {
	if len(cells) == 0 {
		return kernel.OutcomePass, nil
	}
	return s.editTable(cells[0], 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		changed, err := clearCells(document, cells)
		if err == nil && !changed {
			err = errNoChange
		}
		return 0, err
	})
}

// FillTable writes texts, rows of cells' texts, into cell's table from the
// cell on, making the rows and columns they need, as pasting a table into
// one does. The caret goes to the end of the last cell written.
func (s *Session) FillTable(cell model.NodeID, texts [][]string) (kernel.Outcome, error) {
	return s.editTable(cell, 0, func(document *model.DocumentDraft, table Table) (model.NodeID, error) {
		row, column, _ := table.Find(cell)
		filled, err := fillCells(document, table, row, column, texts)
		if err != nil || len(texts) == 0 {
			return 0, err
		}
		last := texts[len(texts)-1]
		return filled.Rows[row+len(texts)-1][column+max(0, len(last)-1)], nil
	})
}

// SelectCell selects cell's whole text.
func (s *Session) SelectCell(cell model.NodeID) error {
	node, ok := s.runtime.Snapshot().Document().Node(cell)
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: cell}
	}
	revision := s.runtime.Revision()
	return s.Select(model.NewTextSelection(
		model.NewTextAnchor(cell, 0, model.AffinityRight, revision),
		model.NewTextAnchor(cell, blockLength(node), model.AffinityRight, revision),
	))
}

// SelectTable selects cell's whole table, as a click on its edge or
// Backspace in it while empty does.
func (s *Session) SelectTable(cell model.NodeID) error {
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return &model.DocumentError{Kind: model.DocumentErrorMissingNode, Node: cell}
	}
	first, okFirst := s.BlockRange(table.First())
	last, okLast := s.BlockRange(table.Last())
	if !okFirst || !okLast {
		return &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	return s.SelectProjected(first.Start, last.End)
}

// errNoChange is an edit of a table that leaves it as it was.
var errNoChange = errorString("editor: the table is as asked")

type errorString string

func (e errorString) Error() string { return string(e) }

// editTable runs edit on cell's table in one step of history, the caret
// put on survivor first when the edit takes the caret's cell away, and
// after it at the end of the cell edit lands in, if any.
func (s *Session) editTable(
	cell, survivor model.NodeID,
	edit func(*model.DocumentDraft, Table) (model.NodeID, error),
) (kernel.Outcome, error) {
	if _, ok := TableOf(s.runtime.Snapshot().Document(), cell); !ok {
		return kernel.OutcomePass, nil
	}
	if survivor != 0 && s.presentation.Selection.Head().Node() != survivor {
		revision := s.runtime.Revision()
		at := model.NewTextAnchor(survivor, 0, model.AffinityRight, revision)
		if err := s.Select(model.NewTextSelection(at, at)); err != nil {
			return 0, err
		}
	}
	var land model.NodeID
	outcome, err := s.dispatchWriting(tableTrigger, "table", func(document *model.DocumentDraft) (bool, error) {
		table, ok := tableAround(document, cell)
		if !ok {
			return false, nil
		}
		next, err := edit(document, table)
		if err == errNoChange {
			return false, nil
		}
		land = next
		return err == nil, err
	})
	return s.landIn(outcome, err, land, false)
}

// landIn puts the caret at the end of land's text once an edit is made;
// whole selects the text instead.
func (s *Session) landIn(outcome kernel.Outcome, err error, land model.NodeID, whole bool) (kernel.Outcome, error) {
	if err != nil || outcome != kernel.OutcomeHandled || land == 0 {
		return outcome, err
	}
	node, ok := s.runtime.Snapshot().Document().Node(land)
	if !ok {
		return outcome, nil
	}
	revision := s.runtime.Revision()
	end := model.NewTextAnchor(land, blockLength(node), model.AffinityRight, revision)
	anchor := end
	if whole {
		anchor = model.NewTextAnchor(land, 0, model.AffinityRight, revision)
	}
	return outcome, s.Select(model.NewTextSelection(anchor, end))
}

// caretRow and caretColumn are where the caret's cell stands in table, 0
// when it is outside.
func (s *Session) caretRow(table Table) int {
	row, _, _ := table.Find(s.presentation.Selection.Head().Node())
	return row
}

func (s *Session) caretColumn(table Table) int {
	_, column, _ := table.Find(s.presentation.Selection.Head().Node())
	return column
}

func emptyColumn(document blockReader, table Table, column int) bool {
	for _, row := range table.Rows {
		if node, ok := document.Node(row[column]); ok && blockLength(node) > 0 {
			return false
		}
	}
	return true
}

func columnHolds(table Table, column int, node model.NodeID) bool {
	for _, row := range table.Rows {
		if row[column] == node {
			return true
		}
	}
	return false
}

// stepCell is Tab and Shift-Tab in a table: the next or the previous
// cell, its text selected, so typing replaces it. Tab on the last cell
// makes a row to go on in. It reports whether the caret was in a table.
func (s *Session) stepCell(step int) (kernel.Outcome, bool, error) {
	cell := s.presentation.Selection.Head().Node()
	table, ok := TableOf(s.runtime.Snapshot().Document(), cell)
	if !ok {
		return kernel.OutcomePass, false, nil
	}
	cells := table.Cells()
	next := slices.Index(cells, cell) + step
	switch {
	case next >= len(cells):
		outcome, err := s.InsertTableRows(cell, len(table.Rows), 1)
		if err == nil && outcome == kernel.OutcomeHandled {
			grown, _ := TableOf(s.runtime.Snapshot().Document(), cell)
			err = s.SelectCell(grown.Rows[len(grown.Rows)-1][0])
		}
		return outcome, true, err
	case next < 0:
		return kernel.OutcomeHandled, true, nil
	}
	return kernel.OutcomeHandled, true, s.SelectCell(cells[next])
}

// growDown is Return in a written last row: a row comes in below it, as a
// table's own edit, so it may reach past the caret's cell, and the caret
// goes down into it. It reports whether it took the Return.
func (s *Session) growDown() (kernel.Outcome, bool, error) {
	cell := s.presentation.Selection.Head().Node()
	document := s.runtime.Snapshot().Document()
	table, ok := TableOf(document, cell)
	if !ok {
		return kernel.OutcomePass, false, nil
	}
	row, column, _ := table.Find(cell)
	if row+1 < len(table.Rows) || emptyRow(document, table.Rows[row]) {
		return kernel.OutcomePass, false, nil
	}
	outcome, err := s.InsertTableRows(cell, len(table.Rows), 1)
	if err == nil && outcome == kernel.OutcomeHandled {
		grown, _ := TableOf(s.runtime.Snapshot().Document(), cell)
		err = s.SelectCell(grown.Rows[len(grown.Rows)-1][column])
	}
	return outcome, true, err
}

// guardTable keeps Backspace and Delete at a cell's edge from joining it to
// what is beside it: at a cell's start, Backspace goes to the end of the
// cell before, and at its end Delete to the start of the next; at the
// start of a block after a table, Backspace goes into the table, and at
// the end of one before it, Delete. Backspace in a table holding nothing
// selects it, for a second press to take it away. An empty block beside a
// table goes as any empty block does. It reports whether it moved.
func (s *Session) guardTable(backward bool) (kernel.Outcome, bool, error) {
	if !s.presentation.VisualCollapsed() {
		return 0, false, nil
	}
	document := s.runtime.Snapshot().Document()
	head := s.presentation.Selection.Head()
	node, ok := document.Node(head.Node())
	if !ok {
		return 0, false, nil
	}
	revision := s.runtime.Revision()
	move := func(to *model.Node, atEnd bool) (kernel.Outcome, bool, error) {
		offset := model.UTF16Offset(0)
		if atEnd {
			offset = blockLength(to)
		}
		at := model.NewTextAnchor(to.ID(), offset, model.AffinityRight, revision)
		return kernel.OutcomeHandled, true, s.Select(model.NewTextSelection(at, at))
	}
	if backward && head.Offset() != 0 || !backward && head.Offset() != blockLength(node) {
		return 0, false, nil
	}
	step := 1
	if backward {
		step = -1
	}
	beside, besideOK := neighborBlock(document, node.ID(), step)
	if kindOf(node).Cell {
		table, _ := TableOf(document, node.ID())
		cells := table.Cells()
		next := slices.Index(cells, node.ID()) + step
		if next >= 0 && next < len(cells) {
			to, _ := document.Node(cells[next])
			return move(to, backward)
		}
		if backward && emptyTable(document, table) {
			return kernel.OutcomeHandled, true, s.SelectTable(node.ID())
		}
		if !besideOK || blockLength(beside) == 0 && kindOf(beside).Plain {
			return kernel.OutcomeHandled, true, nil
		}
		return move(beside, backward)
	}
	if !besideOK || !kindOf(beside).Cell || blockLength(node) == 0 {
		return 0, false, nil
	}
	return move(beside, backward)
}

func emptyTable(document blockReader, table Table) bool {
	for _, row := range table.Rows {
		if !emptyRow(document, row) {
			return false
		}
	}
	return true
}

// tableFlow is each table of the page as the surface sets it out, each
// cell the block of flow it is.
func tableFlow(flow *surface.Flow, document model.DocumentSnapshot) []surface.Table {
	tables := readTables(document)
	if len(tables) == 0 {
		return nil
	}
	ranges := make(map[model.NodeID]model.UTF16Range)
	flow.EachBlock(func(_ int, block surface.Block) bool {
		ranges[block.Node] = block.Range
		return true
	})
	shown := make([]surface.Table, 0, len(tables))
	for _, table := range tables {
		next := surface.Table{
			Header: table.Header, Align: table.Align, Fit: table.Fit,
			Rows: make([][]surface.TableCell, len(table.Rows)),
		}
		whole := true
		for r, row := range table.Rows {
			whole = whole && len(row) == table.Columns()
			for _, id := range row {
				span, ok := ranges[id]
				whole = whole && ok
				next.Rows[r] = append(next.Rows[r], surface.TableCell{Node: id, Range: span})
			}
		}
		if whole {
			shown = append(shown, next)
		}
	}
	return shown
}

// inCell reports whether node is a table's cell.
func (s *Session) inCell(node model.NodeID) bool {
	current, ok := s.runtime.Snapshot().Document().Node(node)
	return ok && kindOf(current).Cell
}

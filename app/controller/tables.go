package controller

import (
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// A table's commands, as its grips, menus and create areas ask for them,
// each one step of history (editor/session_table_commands.go).

// Table is the table cell is in, as the page keeps it.
func (c *DocumentController) Table(cell model.NodeID) (editor.Table, bool) {
	return editor.TableOf(c.session.Runtime().Snapshot().Document(), cell)
}

// CellTexts are the texts of cells, row by row.
func (c *DocumentController) CellTexts(rows [][]model.NodeID) [][]string {
	document := c.session.Runtime().Snapshot().Document()
	texts := make([][]string, len(rows))
	for r, row := range rows {
		for _, id := range row {
			node, _ := document.Node(id)
			text := ""
			if node != nil {
				text = node.Text().String()
			}
			texts[r] = append(texts[r], text)
		}
	}
	return texts
}

func (c *DocumentController) tableCommand(outcome kernel.Outcome, err error) (kernel.Outcome, error) {
	c.resetPreferredColumn()
	return c.finishCommand(outcome, err)
}

func (c *DocumentController) InsertTable(rows [][]string) (kernel.Outcome, error) {
	return c.tableCommand(c.session.InsertTable(rows))
}

func (c *DocumentController) InsertTableRows(cell model.NodeID, at, count int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.InsertTableRows(cell, at, count))
}

func (c *DocumentController) InsertTableColumns(cell model.NodeID, at, count int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.InsertTableColumns(cell, at, count))
}

func (c *DocumentController) DeleteTableRows(cell model.NodeID, first, last int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.DeleteTableRows(cell, first, last))
}

func (c *DocumentController) DeleteTableColumns(cell model.NodeID, first, last int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.DeleteTableColumns(cell, first, last))
}

func (c *DocumentController) DeleteTable(cell model.NodeID) (kernel.Outcome, error) {
	return c.tableCommand(c.session.DeleteTable(cell))
}

func (c *DocumentController) MoveTableRow(cell model.NodeID, from, to int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.MoveTableRow(cell, from, to))
}

func (c *DocumentController) MoveTableColumn(cell model.NodeID, from, to int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.MoveTableColumn(cell, from, to))
}

func (c *DocumentController) ResizeTable(cell model.NodeID, rows, columns int) (kernel.Outcome, error) {
	return c.tableCommand(c.session.ResizeTable(cell, rows, columns))
}

func (c *DocumentController) ShapeTable(cell model.NodeID, change func(*editor.Table)) (kernel.Outcome, error) {
	return c.tableCommand(c.session.ShapeTable(cell, change))
}

func (c *DocumentController) ClearTableCells(cells []model.NodeID) (kernel.Outcome, error) {
	return c.tableCommand(c.session.ClearTableCells(cells))
}

func (c *DocumentController) FillTable(cell model.NodeID, texts [][]string) (kernel.Outcome, error) {
	return c.tableCommand(c.session.FillTable(cell, texts))
}

// SelectCell selects cell's whole text.
func (c *DocumentController) SelectCell(cell model.NodeID) error {
	return c.selectWith(func() error { return c.session.SelectCell(cell) })
}

// SelectTable selects cell's whole table.
func (c *DocumentController) SelectTable(cell model.NodeID) error {
	return c.selectWith(func() error { return c.session.SelectTable(cell) })
}

func (c *DocumentController) selectWith(selectIt func() error) error {
	c.resetPreferredColumn()
	if c.preedit != nil {
		return c.fail(editor.ErrCompositionActive)
	}
	if err := selectIt(); err != nil {
		return c.fail(err)
	}
	c.refreshSource()
	c.refreshVisible()
	c.setStatus(kernel.OutcomeHandled, false, nil)
	return nil
}

// Cells returns the cells of a table picked whole, if any are.
func (c *DocumentController) Cells() (editor.CellSelection, bool) { return c.session.Cells() }

// SelectCells picks cells whole.
func (c *DocumentController) SelectCells(cells editor.CellSelection) error {
	return c.selectWith(func() error { return c.session.SelectCells(cells) })
}

// DropCells lets the cells picked whole go.
func (c *DocumentController) DropCells() { c.session.DropCells() }

// PickLines picks a table's row or column as a grip's click does.
func (c *DocumentController) PickLines(table model.NodeID, kind editor.CellsKind, index int, extend, toggle bool) error {
	return c.selectWith(func() error { return c.session.PickLines(table, kind, index, extend, toggle) })
}

// PickCell picks a cell as an extending or toggling click in it does,
// reporting whether it did.
func (c *DocumentController) PickCell(table model.NodeID, row, column int, extend, toggle bool) (bool, error) {
	picked := false
	err := c.selectWith(func() error {
		var err error
		picked, err = c.session.PickCell(table, row, column, extend, toggle)
		return err
	})
	return picked, err
}

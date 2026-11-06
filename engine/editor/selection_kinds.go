package editor

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// The selection is one of three kinds, each the engine's: text, an object
// (an atomic block's whole line, as a picture), or cells of a table picked
// whole, as a spreadsheet's are. Intents resolve on the kind: Delete
// empties picked cells; Dismiss lets cells go, or leaves an object with the
// caret at its caption's end; Break on an object goes on below it; any other
// intent lets cells go first. Copying reads the kind (SelectedText). The
// view keeps only its gestures, which pick by these methods.

// SelectionKind is what the selection is.
type SelectionKind uint8

const (
	SelectionText SelectionKind = iota
	SelectionObject
	SelectionCells
)

// CellsKind is what a selection of whole cells picks: cells, or rows or
// columns whole.
type CellsKind uint8

const (
	PickedCells CellsKind = iota
	PickedRows
	PickedColumns
)

// CellSelection is cells of one table, named by its first cell, selected
// whole: the rows or the columns Lines names for those kinds, or else
// Cells, each a row and a column. Anchor is where a Shift-click's run
// starts: the cell, or for lines the line, first.
type CellSelection struct {
	Table  model.NodeID
	Kind   CellsKind
	Lines  []int
	Cells  [][2]int
	Anchor [2]int
}

// Has reports whether the cell at row and column is picked.
func (s *CellSelection) Has(row, column int) bool {
	switch s.Kind {
	case PickedRows:
		return slices.Contains(s.Lines, row)
	case PickedColumns:
		return slices.Contains(s.Lines, column)
	}
	return slices.Contains(s.Cells, [2]int{row, column})
}

// Picked are the cells picked in table, row by row, rows with none left
// out.
func (s *CellSelection) Picked(table Table) [][]model.NodeID {
	var rows [][]model.NodeID
	for r, row := range table.Rows {
		var cells []model.NodeID
		for c, id := range row {
			if s.Has(r, c) {
				cells = append(cells, id)
			}
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	return rows
}

// Spans are the rows and the columns the picked cells stand in, in order.
func (s *CellSelection) Spans(table Table) (rows, columns []int) {
	for r, row := range table.Rows {
		for c := range row {
			if s.Has(r, c) {
				if !slices.Contains(rows, r) {
					rows = append(rows, r)
				}
				if !slices.Contains(columns, c) {
					columns = append(columns, c)
				}
			}
		}
	}
	slices.Sort(columns)
	return rows, columns
}

// CellRect picks the cells of table's rectangle between two corners.
func CellRect(table model.NodeID, anchor, head [2]int) CellSelection {
	selection := CellSelection{Table: table, Anchor: anchor}
	for r := min(anchor[0], head[0]); r <= max(anchor[0], head[0]); r++ {
		for c := min(anchor[1], head[1]); c <= max(anchor[1], head[1]); c++ {
			selection.Cells = append(selection.Cells, [2]int{r, c})
		}
	}
	return selection
}

func toggled[T comparable](list []T, item T) []T {
	if index := slices.Index(list, item); index >= 0 {
		return slices.Delete(slices.Clone(list), index, index+1)
	}
	return append(slices.Clone(list), item)
}

// SelectionKind is what the selection is now.
func (s *Session) SelectionKind() SelectionKind {
	if _, ok := s.pickedCells(); ok {
		return SelectionCells
	}
	if _, ok := s.selectedObject(); ok {
		return SelectionObject
	}
	return SelectionText
}

// Cells returns the cells picked whole, if any are.
func (s *Session) Cells() (CellSelection, bool) {
	if _, ok := s.pickedCells(); !ok {
		return CellSelection{}, false
	}
	return *s.cells, true
}

// pickedCells are the picked cells row by row, while their table stands.
func (s *Session) pickedCells() ([][]model.NodeID, bool) {
	if s.cells == nil {
		return nil, false
	}
	table, ok := TableOf(s.runtime.Snapshot().Document(), s.cells.Table)
	if !ok || table.First() != s.cells.Table {
		s.cells = nil
		return nil, false
	}
	return s.cells.Picked(table), true
}

// SelectCells picks cells whole: the text selection goes to the anchor
// cell's text, which typing writes over. Picking none lets them go.
func (s *Session) SelectCells(cells CellSelection) error {
	table, ok := TableOf(s.runtime.Snapshot().Document(), cells.Table)
	if !ok || len(cells.Picked(table)) == 0 {
		s.cells = nil
		return nil
	}
	row, column := cells.Anchor[0], cells.Anchor[1]
	switch cells.Kind {
	case PickedRows:
		column = 0
	case PickedColumns:
		row, column = 0, cells.Anchor[0]
	}
	if row >= len(table.Rows) || column >= len(table.Rows[row]) {
		row, column = 0, 0
	}
	if err := s.SelectCell(table.Rows[row][column]); err != nil {
		return err
	}
	cells.Lines, cells.Cells = slices.Clone(cells.Lines), slices.Clone(cells.Cells)
	s.cells = &cells
	return nil
}

// DropCells lets the cells picked whole go, the text selection staying.
func (s *Session) DropCells() { s.cells = nil }

// PickLines picks the line of kind at index in table, as a grip's click
// does: alone, as the run from the first picked with extend, or added or
// taken back with toggle.
func (s *Session) PickLines(table model.NodeID, kind CellsKind, index int, extend, toggle bool) error {
	current := s.cells
	same := current != nil && current.Table == table && current.Kind == kind
	next := CellSelection{Table: table, Kind: kind, Lines: []int{index}, Anchor: [2]int{index}}
	switch {
	case same && extend:
		from := current.Anchor[0]
		next.Lines, next.Anchor = nil, current.Anchor
		for line := min(from, index); line <= max(from, index); line++ {
			next.Lines = append(next.Lines, line)
		}
	case same && toggle:
		next.Lines, next.Anchor = toggled(current.Lines, index), current.Anchor
		slices.Sort(next.Lines)
	}
	return s.SelectCells(next)
}

// PickCell picks the cell at row and column of table as a click that
// extends or toggles does in it, from the cells picked or the caret's: the
// rectangle from the first when extending, or the cell added or taken back
// when toggling. It reports whether the click picks; in the caret's own
// cell an extending click extends its text instead.
func (s *Session) PickCell(table model.NodeID, row, column int, extend, toggle bool) (bool, error) {
	if !extend && !toggle {
		return false, nil
	}
	shape, ok := TableOf(s.runtime.Snapshot().Document(), table)
	if !ok {
		return false, nil
	}
	current := s.cells
	if current == nil || current.Table != table || current.Kind != PickedCells {
		at, from, ok := shape.Find(s.presentation.Selection.Head().Node())
		if !ok {
			return false, nil
		}
		if at == row && from == column {
			if extend {
				return false, nil
			}
			return true, s.SelectCells(CellSelection{Table: table, Cells: [][2]int{{row, column}}, Anchor: [2]int{row, column}})
		}
		current = &CellSelection{Table: table, Cells: [][2]int{{at, from}}, Anchor: [2]int{at, from}}
	}
	if extend {
		return true, s.SelectCells(CellRect(table, current.Anchor, [2]int{row, column}))
	}
	next := CellSelection{Table: table, Cells: toggled(current.Cells, [2]int{row, column}), Anchor: current.Anchor}
	if !slices.Contains(next.Cells, next.Anchor) && len(next.Cells) > 0 {
		next.Anchor = next.Cells[0]
	}
	return true, s.SelectCells(next)
}

// selectedObject is the atomic block the selection is the whole line of.
func (s *Session) selectedObject() (model.UTF16Range, bool) {
	selected, err := s.VisualSelection()
	if err != nil || selected.Empty() {
		return model.UTF16Range{}, false
	}
	for _, block := range DocumentBlocks(s.runtime.Snapshot().Document()) {
		if !shapeOf(block).atomic() {
			continue
		}
		if line, ok := s.BlockRange(block.ID()); ok && line == selected {
			return line, true
		}
	}
	return model.UTF16Range{}, false
}

// SelectedText is the selection's text as copying takes it: picked cells
// as a spreadsheet copies them, else the page's text across it as a reader
// takes it.
func (s *Session) SelectedText() (string, error) {
	if rows, ok := s.pickedCells(); ok {
		document := s.runtime.Snapshot().Document()
		var text strings.Builder
		for _, row := range rows {
			for index, id := range row {
				if index > 0 {
					text.WriteByte('\t')
				}
				if node, ok := document.Node(id); ok {
					text.WriteString(node.Text().String())
				}
			}
			text.WriteByte('\n')
		}
		return text.String(), nil
	}
	selected, err := s.VisualSelection()
	if err != nil || selected.Empty() {
		return "", err
	}
	return s.ReadableText(selected)
}

// Cut copies the selection's text and takes it away: picked cells are
// emptied, any other selection deleted.
func (s *Session) Cut() (string, EditingOutcome, error) {
	text, err := s.SelectedText()
	if err != nil || text == "" {
		return text, EditingOutcome{Outcome: kernel.OutcomePass}, err
	}
	// Deleting takes the selection as its kind does.
	outcome, err := s.Do(Delete(ByGrapheme, Backward), time.Time{})
	return text, outcome, err
}

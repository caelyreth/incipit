package documentview

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// Cells of a table are selected whole as a spreadsheet's are. A drag
// across cells, or a Shift-click from the caret's cell, takes the
// rectangle between; a Command-click adds a cell, or takes it back. A
// row's grip or a column's selects it whole, a Shift-click on another the
// run from the first, a Command-click adds one or takes it back, and
// their menus act on all of them. Delete empties what is selected, and
// copying gives its texts. Each cell picked fades in, tinted, and what is
// picked together wears one ring about it, which parts as cells leave.

// cellsKind is what a selection of whole cells picks: cells, or rows or
// columns whole.
type cellsKind uint8

const (
	pickedCells cellsKind = iota
	pickedRows
	pickedColumns
)

// CellSelection is cells of one table, named by its first cell, selected
// whole: the rows or the columns Lines names for those kinds, or else
// Cells, each a row and a column. Anchor is where a Shift-click's run
// starts: the cell, or for lines the line, first.
type CellSelection struct {
	Table  model.NodeID
	Kind   cellsKind
	Lines  []int
	Cells  [][2]int
	Anchor [2]int
}

// Has reports whether the cell at row and column is picked.
func (s *CellSelection) Has(row, column int) bool {
	switch s.Kind {
	case pickedRows:
		return slices.Contains(s.Lines, row)
	case pickedColumns:
		return slices.Contains(s.Lines, column)
	}
	return slices.Contains(s.Cells, [2]int{row, column})
}

// Picked are the cells picked in table, row by row, rows with none left
// out.
func (s *CellSelection) Picked(table editor.Table) [][]model.NodeID {
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

// spans are the rows and the columns the picked cells stand in, in order.
func (s *CellSelection) spans(table editor.Table) (rows, columns []int) {
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

// cellRect picks the cells of table's rectangle between two corners.
func cellRect(table model.NodeID, anchor, head [2]int) CellSelection {
	selection := CellSelection{Table: table, Anchor: anchor}
	for r := min(anchor[0], head[0]); r <= max(anchor[0], head[0]); r++ {
		for c := min(anchor[1], head[1]); c <= max(anchor[1], head[1]); c++ {
			selection.Cells = append(selection.Cells, [2]int{r, c})
		}
	}
	return selection
}

// toggled is list with item in it if it was not, and out if it was.
func toggled[T comparable](list []T, item T) []T {
	if index := slices.Index(list, item); index >= 0 {
		return slices.Delete(slices.Clone(list), index, index+1)
	}
	return append(slices.Clone(list), item)
}

// selectCells selects cells whole: the text selection goes to the anchor
// cell's text, which typing writes over. Picking none lets them go.
func (b *Bridge) selectCells(cells CellSelection) {
	table, ok := b.tableShape(cells.Table)
	if !ok || len(cells.Picked(table)) == 0 {
		b.Cells = nil
		return
	}
	row, column := cells.Anchor[0], cells.Anchor[1]
	switch cells.Kind {
	case pickedRows:
		column = 0
	case pickedColumns:
		row, column = 0, cells.Anchor[0]
	}
	if row >= len(table.Rows) || column >= len(table.Rows[row]) {
		row, column = 0, 0
	}
	_ = b.Controller.SelectCell(table.Rows[row][column])
	b.Cells = &cells
}

// pickLines picks the line of kind at index in table, as a grip's click
// with mods does: alone, as the run from the first picked with Shift, or
// added or taken back with Command.
func (b *Bridge) pickLines(table model.NodeID, kind cellsKind, index int, mods ui.Modifiers) {
	current := b.Cells
	same := current != nil && current.Table == table && current.Kind == kind
	next := CellSelection{Table: table, Kind: kind, Lines: []int{index}, Anchor: [2]int{index}}
	switch {
	case same && mods&ui.Shift != 0:
		from := current.Anchor[0]
		next.Lines, next.Anchor = nil, current.Anchor
		for line := min(from, index); line <= max(from, index); line++ {
			next.Lines = append(next.Lines, line)
		}
	case same && mods&ui.Cmd != 0:
		next.Lines, next.Anchor = toggled(current.Lines, index), current.Anchor
		slices.Sort(next.Lines)
	}
	b.selectCells(next)
}

// pickCell picks the cell at row and column of table as a click with
// Shift or Command does in it, from the cells picked or the caret's: the
// rectangle from the first with Shift, or the cell added or taken back
// with Command. It reports whether the click was one of those.
func (b *Bridge) pickCell(table model.NodeID, row, column int, mods ui.Modifiers) bool {
	if mods&(ui.Shift|ui.Cmd) == 0 {
		return false
	}
	shape, ok := b.tableShape(table)
	if !ok {
		return false
	}
	current := b.Cells
	if current == nil || current.Table != table || current.Kind != pickedCells {
		// From the caret's cell, when it is in this table.
		at, from, ok := shape.Find(b.Controller.Session().Selection().Head().Node())
		if !ok {
			return false
		}
		if at == row && from == column {
			// In the caret's own cell, Shift extends its text; Command
			// picks it.
			if mods&ui.Shift != 0 {
				return false
			}
			b.selectCells(CellSelection{Table: table, Cells: [][2]int{{row, column}}, Anchor: [2]int{row, column}})
			return true
		}
		current = &CellSelection{Table: table, Cells: [][2]int{{at, from}}, Anchor: [2]int{at, from}}
	}
	if mods&ui.Shift != 0 {
		b.selectCells(cellRect(table, current.Anchor, [2]int{row, column}))
		return true
	}
	next := CellSelection{Table: table, Cells: toggled(current.Cells, [2]int{row, column}), Anchor: current.Anchor}
	if !slices.Contains(next.Cells, next.Anchor) && len(next.Cells) > 0 {
		next.Anchor = next.Cells[0]
	}
	b.selectCells(next)
	return true
}

// selectedCells are the cells selected whole, row by row.
func (b *Bridge) selectedCells() ([][]model.NodeID, bool) {
	if b == nil || b.Cells == nil {
		return nil, false
	}
	table, ok := b.tableShape(b.Cells.Table)
	if !ok {
		b.Cells = nil
		return nil, false
	}
	return b.Cells.Picked(table), true
}

// cellsText is the selected cells' texts as a spreadsheet copies them.
func (b *Bridge) cellsText() (string, bool) {
	rows, ok := b.selectedCells()
	if !ok {
		return "", false
	}
	var text strings.Builder
	for _, row := range b.Controller.CellTexts(rows) {
		text.WriteString(strings.Join(row, "\t"))
		text.WriteByte('\n')
	}
	return text.String(), true
}

// clearCells empties the selected cells.
func (b *Bridge) clearCells() bool {
	rows, ok := b.selectedCells()
	if !ok {
		return false
	}
	var cells []model.NodeID
	for _, row := range rows {
		cells = append(cells, row...)
	}
	_, _ = b.Controller.ClearTableCells(cells)
	return true
}

// cellsFade is how quickly a picked cell's tint and ring come and go,
// and a table's ring: quicker than a picture's, as a selection's own.
const cellsFade = 26

// CellMotion keeps each picked cell's fade, and each table's ring as it
// is selected whole, across frames.
type CellMotion struct {
	cells  map[cellKey]*motion.Spring
	tables map[model.NodeID]*motion.Spring
}

type cellKey struct {
	table       model.NodeID
	row, column int
}

// step moves the fade of each cell of placed toward whether it is picked,
// and its ring toward whether the table is selected whole, and reports
// whether one is still moving. shown is each cell's fade.
func (m *CellMotion) step(state *State, placed textlayout.PlacedTable, now time.Time, animate bool) (shown func(row, column int) float32, ring float32, moving bool) {
	if m.cells == nil {
		m.cells, m.tables = make(map[cellKey]*motion.Spring), make(map[model.NodeID]*motion.Spring)
	}
	node := placed.Table.Node()
	selection := state.TableCells
	if selection != nil && selection.Table != node {
		selection = nil
	}
	follow := func(spring *motion.Spring, target float32) float32 {
		if !animate {
			*spring = motion.NewUnitSpring(target, now)
		}
		value := spring.Step(target, now, cellsFade)
		moving = moving || spring.Moving()
		return value
	}
	for r := range placed.Rows {
		for c := range placed.Columns {
			key := cellKey{node, r, c}
			target := float32(0)
			if selection != nil && selection.Has(r, c) {
				target = 1
			}
			spring, ok := m.cells[key]
			if !ok {
				if target == 0 {
					continue
				}
				fresh := motion.NewUnitSpring(0, now)
				spring = &fresh
				m.cells[key] = spring
			}
			if follow(spring, target) <= 0.001 && target == 0 {
				delete(m.cells, key)
			}
		}
	}
	for key := range m.cells {
		if key.table == node && (key.row >= len(placed.Rows) || key.column >= len(placed.Columns)) {
			delete(m.cells, key)
		}
	}
	whole := float32(0)
	if covers(state.Frame.Selection(), placed.Table.Range()) {
		whole = 1
	}
	spring, ok := m.tables[node]
	if !ok {
		fresh := motion.NewUnitSpring(0, now)
		spring = &fresh
		m.tables[node] = spring
	}
	ring = follow(spring, whole)
	shown = func(row, column int) float32 {
		if row < 0 || column < 0 || row >= len(placed.Rows) || column >= len(placed.Columns) {
			return 0
		}
		if spring, ok := m.cells[cellKey{node, row, column}]; ok {
			return spring.Value()
		}
		return 0
	}
	return shown, ring, moving
}

// tintPicked tints each picked cell of placed as far as it has faded in.
func tintPicked(p *ui.Painter, state *State, placed textlayout.PlacedTable, cell func(row, column int) ui.Rect, shown func(row, column int) float32) {
	tint := state.Theme.Prose.Selection
	for r := range placed.Rows {
		for c := range placed.Columns {
			if a := shown(r, c); a > 0.001 {
				p.Fill(cell(r, c), tint.Alpha(a*state.Motion.SelectionOpacity()), 0)
			}
		}
	}
}

// ringPicked rings what is picked together: each side of a cell where the
// one beyond is not picked as far, so two cells side by side share one
// ring. It stands within the lines about the cell, which a cell's rect
// begins with, so they still show.
func ringPicked(p *ui.Painter, state *State, placed textlayout.PlacedTable, cell func(row, column int) ui.Rect, shown func(row, column int) float32) {
	ink := state.Theme.Accent.Foreground
	scale := max(1, p.Scale())
	width := 1 / scale * float32(int(scale+0.5))
	for r := range placed.Rows {
		for c := range placed.Columns {
			a := shown(r, c)
			if a <= 0.001 {
				continue
			}
			box := cell(r, c)
			box.X, box.Y, box.W, box.H = box.X+1, box.Y+1, box.W-1, box.H-1
			for _, side := range []struct {
				beyond float32
				line   ui.Rect
			}{
				{shown(r-1, c), ui.Rect{X: box.X, Y: box.Y, W: box.W, H: width}},
				{shown(r+1, c), ui.Rect{X: box.X, Y: box.Y + box.H - width, W: box.W, H: width}},
				{shown(r, c-1), ui.Rect{X: box.X, Y: box.Y, W: width, H: box.H}},
				{shown(r, c+1), ui.Rect{X: box.X + box.W - width, Y: box.Y, W: width, H: box.H}},
			} {
				if edge := a - side.beyond; edge > 0.001 {
					p.Fill(side.line, ink.Alpha(0.85*edge), 0)
				}
			}
		}
	}
}

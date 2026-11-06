package documentview

import (
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

// pickedCells are the cells the engine has picked whole, if any are, for
// the frame to show.
func (b *Bridge) pickedCells() *editor.CellSelection {
	if b == nil || b.Controller == nil {
		return nil
	}
	cells, ok := b.Controller.Cells()
	if !ok {
		return nil
	}
	return &cells
}

// hasCells reports whether cells are picked whole.
func (b *Bridge) hasCells() bool { return b.pickedCells() != nil }

// pickLines picks a grip's line as its click with mods does: alone, the
// run from the first with Shift, added or taken back with Command.
func (b *Bridge) pickLines(table model.NodeID, kind editor.CellsKind, index int, mods ui.Modifiers) {
	_ = b.Controller.PickLines(table, kind, index, mods&ui.Shift != 0, mods&ui.Cmd != 0)
}

// pickCell picks a cell as a Shift- or Command-click in it does, reporting
// whether the click picked.
func (b *Bridge) pickCell(table model.NodeID, row, column int, mods ui.Modifiers) bool {
	picked, _ := b.Controller.PickCell(table, row, column, mods&ui.Shift != 0, mods&ui.Cmd != 0)
	return picked
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

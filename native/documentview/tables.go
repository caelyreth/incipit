package documentview

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/scrollfx"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A table on the page is made where it grows, as the sheet list makes
// sheets. The pointer resting by a table opens a dashed area below it that
// makes a row, the page below gliding down for it, and one beside it that
// makes a column. The table's edges hold what changes its shape, its cells
// only its text: resting in the gutter before it by a border between two
// rows, or above it by one between two columns, opens an area there, the
// rows or columns gliding apart. What an area makes stands in its place.
// Over a row or a column, a grip in the gutter selects its cells whole,
// and dragging it carries it among the others (carry.go); the grip at the corner where
// they meet moves the table among the page's blocks (blocks.go), and the
// corner across drags it larger or smaller, its new size outlined and
// named as it goes, kept on letting go, and Escape puts it back. The
// borders between columns set their widths (columns.go). A drag
// across cells selects them whole: Delete empties them, copying gives
// their texts between tabs, and typing writes over the first. A secondary
// click on a cell opens the table's menu.

const (
	// tableReach is how near a table, in DIPs, the pointer calls up the
	// areas that make its rows and columns, and tableHold how near it keeps
	// them once open, so a pointer wandering a little past does not shut
	// them; tableBorder is how near a border between two rows or columns,
	// in the gutter, calls up one there.
	tableReach  = 28
	tableHold   = 56
	tableBorder = 8
	// gripLong and gripShort size a grip, in DIPs at the default UI size,
	// gripIcon its icon, and gripOff sets it off what it moves.
	gripLong, gripShort, gripIcon, gripOff = 22, 16, 14, 4
	// cornerSize is the corner's handle.
	cornerSize = 10
	// gapStride numbers a border gap by its table and its place, so two
	// tables' gaps never share a number.
	gapStride = 1 << 12
)

// CellRange is a selection of whole cells: the rectangle between two
// corners of a table, named by its first cell.
type CellRange struct {
	Table        model.NodeID
	Anchor, Head [2]int
}

// Bounds are the range's first and last row and column.
func (r CellRange) Bounds() (top, left, bottom, right int) {
	top, bottom = min(r.Anchor[0], r.Head[0]), max(r.Anchor[0], r.Head[0])
	left, right = min(r.Anchor[1], r.Head[1]), max(r.Anchor[1], r.Head[1])
	return
}

// Cells are the range's cells in table, row by row.
func (r CellRange) Cells(table editor.Table) [][]model.NodeID {
	top, left, bottom, right := r.Bounds()
	var rows [][]model.NodeID
	for row := top; row <= min(bottom, len(table.Rows)-1); row++ {
		cells := table.Rows[row]
		rows = append(rows, append([]model.NodeID(nil), cells[left:min(right+1, len(cells))]...))
	}
	return rows
}

// tableUI is what the page keeps of its tables across frames.
type tableUI struct {
	// below opens the areas by a table, numbered by its place; rows and
	// columns those on a border, numbered by gapStride.
	below, rows, columns components.CreateGap
	// room is what each table opens, by its first cell, as the view sets
	// it each frame for the next layout.
	room map[model.NodeID]surface.TableRoom
	// grips are how far each table's grips have come in, by its first
	// cell, as the pointer comes by it.
	grips map[model.NodeID]float32
	// cells are the cells seen last frame, and growing the rows and
	// columns coming in, by their first cell.
	cells                    map[model.NodeID]bool
	tables                   map[model.NodeID]bool
	growingRows, growingCols map[model.NodeID]motion.Spring
	drag                     *tableDrag
	// held is the grip a press holds, kept under it until it lets go
	// though the pointer leaves its row or column.
	held *tableDrag
	// press is the cell a press began in, for a drag across cells.
	press      model.NodeID
	pressTable model.NodeID
	// pointer is where the pointer was in the page's content, as the view
	// last read it.
	pointer   ui.Point
	pointerIn bool
	// made is the row or column a create area made, taking the area's
	// place; generation renumbers a table's room as one is used, so the
	// room the area had goes at once, the new row or column standing in it.
	made       *tableMade
	generation map[model.NodeID]int
	// scroll is how far each table wider than its room is scrolled
	// across, by its first cell, and wheeled those the wheel moved this
	// frame, which follow it at once; caret is where the caret was, and
	// caretGrid how wide its table's grid, so one moved into a cell out
	// of its room, or left out of it as the grid changes, scrolls it in.
	scroll    map[model.NodeID]float32
	wheeled   map[model.NodeID]bool
	caret     int
	caretGrid float32
	// pull is each such table's give past the ends of its scroll, as the
	// page's past its own.
	pull map[model.NodeID]*scrollfx.Elastic
	// sizing is a drag of a column's border under way, and border the
	// border the pointer is on (columns.go).
	sizing *columnSizing
	border columnBorder
}

// tableMade is a row or column a create area made: it comes in at the
// area's size, the area's outline fading into it.
type tableMade struct {
	table  model.NodeID
	column bool
	index  int
	// from is the share of its own size it comes in at.
	from float32
	at   time.Time
}

// madeFade is how long a made row or column shows the area it was.
const madeFade = 2 * components.MotionNormal

// madeShare is how far the made row or column at index of table comes in
// from, if one is: its area's size, as a share of its own.
func (t *tableUI) madeShare(table model.NodeID, column bool, index int) (float32, bool) {
	if m := t.made; m != nil && m.table == table && m.column == column && m.index == index {
		return m.from, true
	}
	return 0, false
}

type tableDragKind uint8

const (
	dragRow tableDragKind = iota + 1
	dragColumn
	dragCorner
)

// tableDrag is a grip a press holds, and the corner while it is dragged:
// how far the pointer has gone, and the rows and columns it sets.
type tableDrag struct {
	table         model.NodeID
	kind          tableDragKind
	from          int
	moved         ui.Point
	rows, columns int
}

// tableShape reads the page's table at its first cell.
func (b *Bridge) tableShape(node model.NodeID) (editor.Table, bool) {
	if b == nil || b.Controller == nil {
		return editor.Table{}, false
	}
	return b.Controller.Table(node)
}

// tabledFlow is flow with each table as the view opens it, and each new
// row and column coming in. It reports whether one is still coming.
func (e *Element) tabledFlow(flow *surface.Flow, now time.Time, animate bool) (*surface.Flow, bool) {
	tables := flow.Tables()
	if len(tables) == 0 {
		e.tables.cells, e.tables.tables = nil, nil
		e.tables.growingRows, e.tables.growingCols = nil, nil
		return flow, false
	}
	cells := map[model.NodeID]bool{}
	known := map[model.NodeID]bool{}
	rows, columns := map[model.NodeID]motion.Spring{}, map[model.NodeID]motion.Spring{}
	moving := false
	grow := func(springs map[model.NodeID]motion.Spring, from map[model.NodeID]motion.Spring, key model.NodeID, fresh bool, start float32) float32 {
		spring, ok := from[key]
		if !ok {
			if !fresh || !animate {
				return 1
			}
			spring = motion.NewUnitSpring(start, now)
		}
		spring.Step(1, now, figureFade)
		if spring.Moving() {
			springs[key] = spring
			moving = true
		}
		return spring.Value()
	}
	for index := range tables {
		table := &tables[index]
		node := table.Node()
		known[node] = true
		seen := e.tables.tables[node]
		room := e.tables.room[node]
		if sizing := e.tables.sizing; sizing != nil && sizing.table == node {
			// A border's drag sets the widths it would keep as it goes.
			table.Widths = sizing.widths
		}
		room.RowShown = make([]float32, len(table.Rows))
		room.ColumnShown = make([]float32, table.Columns())
		for r, row := range table.Rows {
			fresh := seen
			for _, cell := range row {
				cells[cell.Node] = true
				fresh = fresh && !e.tables.cells[cell.Node]
			}
			start, _ := e.tables.madeShare(node, false, r)
			room.RowShown[r] = grow(rows, e.tables.growingRows, row[0].Node, fresh, start)
		}
		for c := range table.Columns() {
			fresh := seen
			for _, row := range table.Rows {
				fresh = fresh && !e.tables.cells[row[c].Node]
			}
			start, _ := e.tables.madeShare(node, true, c)
			room.ColumnShown[c] = grow(columns, e.tables.growingCols, table.Rows[0][c].Node, fresh, start)
		}
		e.tableApart(*table, &room)
		table.Room = room
	}
	e.tables.cells, e.tables.tables = cells, known
	e.tables.growingRows, e.tables.growingCols = rows, columns
	opened, err := flow.WithTables(tables)
	if err != nil {
		return flow, moving
	}
	return opened, moving
}

// followTables reads where the pointer is by the page's tables, as the
// last frame set them out, opens the areas it calls up, and sets the room
// they take for the next layout.
func (e *Element) followTables(c *ui.Context, box ui.Element, animate bool) {
	layout := e.current
	if layout == nil {
		e.tables.room = nil
		return
	}
	placed := layout.Tables()
	scale := e.uiScale()
	metrics := theme.DefaultControls.Scaled(e.chromeSize())
	gap := theme.Space06.DIPs(e.chromeSize())
	inner := theme.Space08.DIPs(e.chromeSize())
	border := tableBorder * scale
	nearAt, nearOpen := e.tables.below.Open()
	point, inside := e.pagePointer(box)
	e.tables.pointer, e.tables.pointerIn = point, inside
	near, rowAt, columnAt := -1, -1, -1
	rowNear, columnNear := float32(0), float32(0)
	free := inside && e.pointer == pointerIdle && e.tables.drag == nil && e.tables.held == nil &&
		e.tables.sizing == nil && e.carry == nil && !e.blocks.holding
	if e.carry != nil {
		// A carry chooses where to land: nothing is made meanwhile.
		e.tables.below.Close()
		e.tables.rows.Close()
		e.tables.columns.Close()
	}
	e.followBorder(box, point, free)
	for index, table := range placed {
		reach := tableReach * scale
		if nearOpen && nearAt == index {
			reach = tableHold * scale
		}
		// The table, its grips above and before it, the column area
		// beside it and the row area below.
		area := table.Box
		area.X -= reach
		area.Y -= reach
		area.W += 2*reach + gap + metrics.Height
		area.H += table.Below.H + 2*reach
		if !free || !contains(area, point) {
			continue
		}
		near = index
		// Room between rows is called up from the gutter before the
		// table, where its rows' grips are, and between columns from the
		// gutter above it, where its columns' are: never from over its
		// cells, where the pointer is for the text. An open gap holds
		// while the pointer is on it, the border's reach around it.
		gutter := tableReach * scale
		inRows := point.X >= table.Box.X-gutter && point.X < table.Box.X
		inColumns := point.Y >= table.Box.Y-gutter && point.Y < table.Box.Y &&
			point.X >= table.Box.X && point.X <= table.Box.X+table.Box.W
		rowGap, columnGap := grown(table.RowGap, 0, border), grown(table.ColumnGap, border, 0)
		rowGap.X, rowGap.W = rowGap.X-gutter, rowGap.W+gutter
		columnGap.Y, columnGap.H = columnGap.Y-gutter, columnGap.H+gutter
		for r := 1; r < len(table.Rows); r++ {
			if inRows && abs(point.Y-table.Rows[r]) <= border || contains(rowGap, point) && table.RowGap.Y < table.Rows[r] {
				rowAt, rowNear = index*gapStride+r, abs(point.Y-table.Rows[r])
				break
			}
		}
		for column := 1; column < len(table.Columns); column++ {
			if inColumns && abs(point.X-table.Columns[column]) <= border || contains(columnGap, point) && table.ColumnGap.X < table.Columns[column] {
				columnAt, columnNear = index*gapStride+column, abs(point.X-table.Columns[column])
				break
			}
		}
	}
	// One gap opens at a time: by a crossing of a row's border and a
	// column's, the one open already, else the nearer border's.
	_, rowsOpen := e.tables.rows.Open()
	_, columnsOpen := e.tables.columns.Open()
	switch {
	case rowsOpen:
		columnAt = -1
	case columnsOpen:
		rowAt = -1
	case rowAt >= 0 && columnAt >= 0 && rowNear <= columnNear:
		columnAt = -1
	case rowAt >= 0 && columnAt >= 0:
		rowAt = -1
	}
	if near < 0 {
		// The pointer has left what it made.
		e.pointerHold = false
	}
	nearAt, nearOpen = e.tables.below.Follow(c, near)
	rowGap, rowOpen := e.tables.rows.Follow(c, rowAt)
	columnGap, columnOpen := e.tables.columns.Follow(c, columnAt)
	// The open areas hold their gaps while the pointer is on them.
	for _, table := range placed {
		if contains(grown(table.RowGap, 0, border), point) && inside && rowOpen {
			e.tables.rows.HoldOver(true)
		}
		if contains(grown(table.ColumnGap, border, 0), point) && inside && columnOpen {
			e.tables.columns.HoldOver(true)
		}
	}
	room := make(map[model.NodeID]surface.TableRoom, len(placed))
	step, between := metrics.Height+gap, metrics.Height+2*inner
	grips := make(map[model.NodeID]float32, len(placed))
	follow := func(key any, target float32) float32 {
		if !animate {
			return target
		}
		return components.Follow(box, key, target, components.MotionNormal)
	}
	if m := e.tables.made; m != nil && c.Now().Sub(m.at) > madeFade {
		e.tables.made = nil
	}
	lift := float32(0)
	for index, table := range placed {
		node := table.Table.Node()
		generation := e.tables.generation[node]
		previous := e.tables.room[node]
		next := surface.TableRoom{Row: previous.Row, Column: previous.Column}
		open := func(is bool, size float32) float32 {
			if is {
				return size
			}
			return 0
		}
		next.Below = follow(tableKey{node, "below", generation}, open(nearOpen && nearAt == index, step))
		grips[node] = follow(tableKey{node, "grips", 0}, open(near == index || e.tables.drag != nil || e.tables.held != nil, 1))
		if rowOpen && rowGap/gapStride == index {
			next.Row = rowGap % gapStride
		}
		next.RowRoom = follow(tableKey{node, "row", generation}, open(rowOpen && rowGap/gapStride == index, between))
		// A gap between rows opens about the border the pointer is on,
		// the page lifting by half of it.
		lift += next.RowRoom / 2
		if columnOpen && columnGap/gapStride == index {
			next.Column = columnGap % gapStride
		}
		next.Scroll = e.tables.scroll[node]
		if animate && !e.tables.wheeled[node] {
			next.Scroll = components.Follow(box, tableKey{node, "scroll", 0}, next.Scroll, components.MotionNormal)
		} else if animate {
			next.Scroll = box.Jump(tableKey{node, "scroll", 0}, next.Scroll)
		}
		if pull := e.tables.pull[node]; pull != nil {
			offset, pulling := pull.Step(c.Now(), animate)
			next.Pull = -offset
			if pulling {
				c.AnimationFrame()
			}
		}
		next.ColumnRoom = follow(tableKey{node, "column", generation}, open(columnOpen && columnGap/gapStride == index, between))
		room[node] = next
	}
	e.tables.room, e.tables.grips = room, grips
	e.tables.wheeled = nil
	e.liftTo = lift
}

// tableReaches lays what of a table reaches past the column into the
// margins over the page as its own, so the page takes a press there as
// on the rest of its cells: the page's input comes before its children's.
func (e *Element) tableReaches(c *ui.Context) {
	offset := e.TopInset - e.visibleScroll()
	width := e.current.Width()
	for _, placed := range e.current.Tables() {
		box := placed.Box
		box.Y += offset
		for index, part := range []ui.Rect{
			{X: box.X, Y: box.Y, W: min(box.W, -box.X), H: box.H},
			{X: max(box.X, width), Y: box.Y, W: box.X + box.W - max(box.X, width), H: box.H},
		} {
			if part.W > 0.5 {
				cursor := ui.CursorText
				if e.tables.border.on && e.tables.border.table == placed.Table.Node() {
					cursor = ui.CursorResizeEW
				}
				ui.Box(c.Key(fmt.Sprintf("table-%d-reach-%d", placed.Table.Node(), index))).
					Absolute().Left(part.X).Top(part.Y).Width(part.W).Height(part.H).
					Cursor(cursor)
			}
		}
	}
}

// tableSpread is how far a table wider than the column may reach into
// each margin, leaving the gutters their grips and areas stand in.
func (e *Element) tableSpread() float32 {
	metrics := theme.DefaultControls.Scaled(e.chromeSize())
	return max(0, e.Margin-tableReach*e.uiScale()-metrics.Height)
}

// wheelTable scrolls the table under the wheel across, when it is wider
// than its room, its ends giving as the page's do, and reports whether it
// took the wheel.
func (e *Element) wheelTable(event ui.InputEvent) bool {
	if e.current == nil || event.DX == 0 {
		return false
	}
	point := e.contentPoint(event.X, event.Y)
	for _, placed := range e.current.Tables() {
		if placed.Overflow <= 0 || !contains(placed.Box, point) {
			continue
		}
		node := placed.Table.Node()
		if e.tables.scroll == nil {
			e.tables.scroll = map[model.NodeID]float32{}
		}
		if e.tables.wheeled == nil {
			e.tables.wheeled = map[model.NodeID]bool{}
		}
		if e.tables.pull == nil {
			e.tables.pull = map[model.NodeID]*scrollfx.Elastic{}
		}
		pull := e.tables.pull[node]
		if pull == nil {
			pull = &scrollfx.Elastic{}
			e.tables.pull[node] = pull
		}
		input := scrollfx.Phase(event.Precise, event.Momentum)
		e.tables.scroll[node] = elasticScroll(pull, e.tables.scroll[node], event.DX, 0, placed.Overflow, placed.Box.W, e.UIFontSize, input)
		e.tables.wheeled[node] = true
		return true
	}
	return false
}

// revealCell scrolls the table the caret has moved into across, as little
// as shows its caret within its room.
func (e *Element) revealCell(head int) {
	if e.current == nil || e.tables.sizing != nil {
		return
	}
	caret := e.current.Caret(model.UTF16Offset(head))
	placed, ok := e.current.TableAt(caret.Y)
	grid := float32(0)
	if ok {
		grid = placed.Grid.W
	}
	// The caret moving, or its table's grid changing width under it, as
	// a column's is set, brings its cell in.
	if head == e.tables.caret && grid == e.tables.caretGrid {
		return
	}
	e.tables.caret, e.tables.caretGrid = head, grid
	if !ok || placed.Overflow <= 0 {
		return
	}
	node := placed.Table.Node()
	pad := e.Prose.LineHeight()
	scroll := placed.Scroll
	switch {
	case caret.X < placed.Box.X+pad:
		scroll -= placed.Box.X + pad - caret.X
	case caret.X > placed.Box.X+placed.Box.W-pad:
		scroll += caret.X - (placed.Box.X + placed.Box.W - pad)
	default:
		return
	}
	if e.tables.scroll == nil {
		e.tables.scroll = map[model.NodeID]float32{}
	}
	e.tables.scroll[node] = max(0, min(scroll, placed.Overflow))
}

type tableKey struct {
	node       model.NodeID
	part       string
	generation int
}

// tableControls places what makes and moves a table's rows and columns
// over the page, where the last frame set the table out.
func (e *Element) tableControls(c *ui.Context, animate bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil || e.current == nil {
		return
	}
	e.tableReaches(c)
	scale := e.uiScale()
	metrics := theme.DefaultControls.Scaled(e.chromeSize())
	gap := theme.Space06.DIPs(e.chromeSize())
	inner := theme.Space08.DIPs(e.chromeSize())
	offset := e.TopInset - e.visibleScroll()
	nearAt, nearOpen := e.tables.below.Open()
	// What the pointer is on keeps open what it is on, wherever it is
	// set, as beside the page in its margin.
	var overNear, overRow, overColumn bool
	defer func() {
		if overNear {
			e.tables.below.HoldOver(true)
		}
		if overRow {
			e.tables.rows.HoldOver(true)
		}
		if overColumn {
			e.tables.columns.HoldOver(true)
		}
	}()
	for index, placed := range e.current.Tables() {
		table := placed.Table
		first := table.Node()
		at := func(r ui.Rect) ui.Rect { r.Y += offset; return r }
		// What an area makes stands where the area was, at its size, as a
		// share of the size it settles at.
		rowShare := func(size float32) float32 {
			return min(1, size/max(1, placed.Heights[len(placed.Heights)-1]))
		}
		columnShare := func(size float32) float32 {
			return min(1, size*float32(table.Columns()+1)/max(1, placed.Box.W))
		}
		made := func(gap *components.CreateGap, column bool, index int, from float32) {
			gap.Use()
			e.focusPending = true
			// The page keeps where the gap lifted it, the row standing in
			// the gap's place, and comes back down as it settles.
			e.lift, e.liftTo = 0, 0
			if e.tables.generation == nil {
				e.tables.generation = map[model.NodeID]int{}
			}
			e.tables.generation[first]++
			e.tables.made = &tableMade{table: first, column: column, index: index, from: from, at: c.Now()}
			e.pointerHold = true
		}
		// Below the table, a row; beside it, a column.
		if below := at(placed.Below); below.H > 1 {
			reveal := min(1, below.H/(metrics.Height+gap))
			area := components.CreateArea{
				Key: fmt.Sprintf("table-%d-new-row", first), Label: "New row",
				Reveal: reveal, LabelReveal: components.Smoothstep(reveal), Partial: true, Animate: animate,
				Action: func() {
					made(&e.tables.below, false, len(table.Rows), rowShare(below.H))
					_, _ = bridge.Controller.InsertTableRows(first, len(table.Rows), 1)
				},
			}.View(c, e.Theme)
			box := ui.Rect{X: below.X, Y: below.Y + gap, W: below.W, H: max(1, below.H-gap)}
			area.Absolute().Left(box.X).Top(box.Y).Width(box.W).Height(box.H).Opacity(reveal)
			e.control(box)
			overNear = overNear || area.Hovered()
		}
		if nearOpen && nearAt == index {
			grid := at(placed.Box)
			reveal := min(1, placed.Below.H/(metrics.Height+gap))
			area := components.CreateArea{
				Key: fmt.Sprintf("table-%d-new-column", first), Label: "New column",
				Compact: true, Animate: animate,
				Action: func() {
					made(&e.tables.below, true, table.Columns(), 0)
					_, _ = bridge.Controller.InsertTableColumns(first, table.Columns(), 1)
				},
			}.View(c, e.Theme)
			box := ui.Rect{X: grid.X + grid.W + gap, Y: grid.Y, W: metrics.Height, H: grid.H}
			area.Absolute().Left(box.X).Top(box.Y).Width(box.W).Height(box.H).Opacity(reveal).
				Translate((1-reveal)*-gap, 0)
			e.control(box)
			overNear = overNear || area.Hovered()
		}
		if gapBox := at(placed.RowGap); gapBox.H > 1 {
			row := e.tables.room[first].Row
			reveal := min(1, gapBox.H/(metrics.Height+2*inner))
			area := components.CreateArea{
				Key: fmt.Sprintf("table-%d-row-gap", first), Label: "New row",
				Reveal: reveal, LabelReveal: components.Smoothstep(reveal), Partial: true, Animate: animate,
				Action: func() {
					made(&e.tables.rows, false, row, rowShare(gapBox.H))
					_, _ = bridge.Controller.InsertTableRows(first, row, 1)
				},
			}.View(c, e.Theme)
			box := ui.Rect{X: gapBox.X, Y: gapBox.Y + inner, W: gapBox.W, H: max(1, gapBox.H-2*inner)}
			area.Absolute().Left(box.X).Top(box.Y).Width(box.W).Height(box.H).Opacity(reveal)
			e.control(box)
			overRow = overRow || area.Hovered()
		}
		if gapBox := at(placed.ColumnGap); gapBox.W > 1 {
			column := e.tables.room[first].Column
			reveal := min(1, gapBox.W/(metrics.Height+2*inner))
			area := components.CreateArea{
				Key: fmt.Sprintf("table-%d-column-gap", first), Label: "New column",
				Compact: true, Reveal: reveal, Partial: true, Animate: animate,
				Action: func() {
					made(&e.tables.columns, true, column, columnShare(gapBox.W))
					_, _ = bridge.Controller.InsertTableColumns(first, column, 1)
				},
			}.View(c, e.Theme)
			box := ui.Rect{X: gapBox.X + inner, Y: gapBox.Y, W: max(1, gapBox.W-2*inner), H: gapBox.H}
			area.Absolute().Left(box.X).Top(box.Y).Width(box.W).Height(box.H).Opacity(reveal)
			e.control(box)
			overColumn = overColumn || area.Hovered()
		}
		overNear = e.tableGrips(c, bridge, placed, scale, animate) || overNear
	}
}

// tableGrips places the grips of the row and column under the pointer,
// and the table's corner, coming in as the pointer comes by the table. It
// reports whether the pointer is on one.
func (e *Element) tableGrips(c *ui.Context, bridge *Bridge, placed textlayout.PlacedTable, scale float32, animate bool) bool {
	table := placed.Table
	first := table.Node()
	drag := e.tables.drag
	if drag != nil && drag.table != first {
		drag = nil
	}
	shown := e.tables.grips[first]
	if drag == nil && shown < 0.01 {
		return false
	}
	row, column := e.hoveredCell(placed)
	if drag == nil && e.tables.held != nil && e.tables.held.table == first {
		drag = e.tables.held
	}
	if drag != nil {
		switch drag.kind {
		case dragRow:
			row = drag.from
		case dragColumn:
			column = drag.from
		}
	}
	long, short, off := gripLong*scale, gripShort*scale, gripOff*scale
	over := false
	grip := func(key, label string, r ui.Rect, kind tableDragKind, from int) {
		icon := components.IconGrip
		switch kind {
		case dragColumn:
			icon = components.IconGripAcross
		case dragCorner:
			icon = ""
		}
		handle := e.grip(c, fmt.Sprintf("table-%d-%s", first, key), label, r, shown, icon, animate)
		now := c.Now()
		over = over || handle.Hovered() || handle.Pressed()
		// A grip stays held until its press, and what it carries, lets go.
		_, _, pressed := handle.Dragged()
		if pressed {
			e.tables.held = &tableDrag{table: first, kind: kind, from: from}
		} else if held := e.tables.held; held != nil && held.table == first && held.kind == kind && e.carry == nil {
			e.tables.held = nil
		}
		e.followGrip(handle, bridge, placed, kind, from, now)
	}
	if row >= 0 && row < len(placed.Rows) {
		box := ui.Rect{
			X: placed.Box.X - off - short, Y: placed.Rows[row] + (placed.Heights[row]-long)/2,
			W: short, H: long,
		}
		grip("row", fmt.Sprintf("Row %d", row+1), box, dragRow, row)
	}
	if column >= 0 && column < len(placed.Columns) {
		box := ui.Rect{
			// A column scrolled partly out of the room keeps its grip in it.
			X: max(placed.Box.X, min(placed.Columns[column]+(placed.Widths[column]-long)/2, placed.Box.X+placed.Box.W-long)),
			Y: placed.Box.Y - off - short,
			W: long, H: short,
		}
		grip("column", fmt.Sprintf("Column %d", column+1), box, dragColumn, column)
	}
	size := cornerSize * scale
	corner := ui.Rect{
		X: placed.Box.X + placed.Box.W - size/2, Y: placed.Box.Y + placed.Box.H - size/2,
		W: size, H: size,
	}
	grip("corner", "Resize table", corner, dragCorner, 0)
	return over
}

// grip places a handle over the page at r, in the page's content: its
// icon on nothing at rest, as the app's quiet buttons are, a tint coming in
// under the pointer and the accent as it is held. Every grip of the page
// is one of these, a block's, a table's and its rows' and columns'; one
// called up again by key glides there from where it was, and one that was
// not there, or there out of sight, comes in where it is. The corner's,
// which resizes rather than moves, is a knob without an icon.
func (e *Element) grip(c *ui.Context, key, label string, r ui.Rect, shown float32, icon components.IconName, animate bool) ui.Element {
	handle := ui.ButtonBase(c.Key(key)).Label(label).FocusRing(false).Opacity(shown).
		HandleInput(e.handleGripInput)
	offset := e.TopInset - e.visibleScroll()
	if animate {
		x := components.Follow(handle, "x", r.X, components.MotionNormal)
		y := components.Follow(handle, "y", r.Y, components.MotionNormal)
		if top := y + offset; top+r.H < 0 || top > e.lastHeight {
			x, y = handle.Jump("x", r.X), handle.Jump("y", r.Y)
		}
		r.X, r.Y = x, y
	}
	r.Y += offset
	handle.Absolute().Left(r.X).Top(r.Y).Width(r.W).Height(r.H)
	e.control(r)
	held := handle.Pressed()
	hover := components.HoverAmount(handle, handle.Hovered() || held, animate)
	if icon == "" {
		handle.Radius(min(r.W, r.H)/2).
			Background(components.Blend(e.Theme.Surface.Elevated, e.Theme.Accent.Foreground, hover)).
			Border(1, components.Blend(e.Theme.Border.Default, e.Theme.Accent.Foreground, hover)).
			Shadow(0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.2)).
			Cursor(ui.CursorResizeNWSE)
		return handle
	}
	cursor := ui.CursorGrab
	if held {
		cursor = ui.CursorGrabbing
	}
	ink := components.Blend(e.Theme.Icon.Muted, e.Theme.Icon.Default, hover)
	if held || e.carry != nil {
		ink = e.Theme.Accent.Foreground
	}
	metrics := theme.DefaultControls.Scaled(e.chromeSize())
	handle.Radius(metrics.Radius).
		Background(e.Theme.GhostElement.Hover.Alpha(hover)).
		Cursor(cursor).
		Children(func() {
			ui.Icon(c, icon.SVG()).FontSize(gripIcon * e.uiScale()).TextColor(ink)
		})
	return handle
}

// followGrip follows a press on a grip: a click selects the row or column
// whole, a drag carries it among the others (carry.go), and the corner's
// drag resizes the table.
func (e *Element) followGrip(handle ui.Element, bridge *Bridge, placed textlayout.PlacedTable, kind tableDragKind, from int, now time.Time) {
	table := placed.Table
	first := table.Node()
	event, moved := e.followGripGesture(handle, fmt.Sprintf("table-%d-%d", first, kind))
	if kind != dragCorner {
		if event == gripStart {
			if kind == dragRow {
				e.startCarry(bridge, carryRow, table.Rows[from][0].Node, from, moved)
			} else {
				e.startCarry(bridge, carryColumn, table.Rows[0][from].Node, from, moved)
			}
		}
		if event == gripEnd {
			e.dropCarry(bridge, false, now)
			return
		}
		if event == gripCancel {
			e.dropCarry(bridge, true, now)
			return
		}
	} else if event == gripStart || event == gripMove {
		drag := e.tables.drag
		if drag == nil || drag.table != first || drag.kind != kind {
			drag = &tableDrag{table: first, kind: kind, from: from,
				rows: len(table.Rows), columns: table.Columns()}
			e.tables.drag = drag
		}
		drag.moved = moved
		cellW := placed.Grid.W / float32(max(1, table.Columns()))
		cellH := placed.Box.H / float32(max(1, len(table.Rows)))
		drag.columns = max(1, table.Columns()+int(math.Round(float64(drag.moved.X/cellW))))
		drag.rows = max(1, len(table.Rows)+int(math.Round(float64(drag.moved.Y/cellH))))
	}
	if kind == dragCorner {
		if event == gripCancel {
			e.tables.drag = nil
			return
		}
		if event == gripEnd {
			drag := e.tables.drag
			e.tables.drag = nil
			e.focusPending = true
			if drag != nil {
				_, _ = bridge.Controller.ResizeTable(first, drag.rows, drag.columns)
			}
			return
		}
	}
	if event != gripClick || e.tables.drag != nil || e.carry != nil {
		return
	}
	e.focusPending = true
	switch kind {
	case dragRow:
		bridge.selectCells(CellRange{Table: first, Anchor: [2]int{from, 0}, Head: [2]int{from, table.Columns() - 1}})
	case dragColumn:
		bridge.selectCells(CellRange{Table: first, Anchor: [2]int{0, from}, Head: [2]int{len(table.Rows) - 1, from}})
	case dragCorner:
		_ = bridge.Controller.SelectTable(first)
	}
}

// hoveredCell is the row and column under the pointer in placed, -1 for
// each it is outside of, as the grips are taken in as well.
func (e *Element) hoveredCell(placed textlayout.PlacedTable) (int, int) {
	point, ok := e.pointerPoint()
	if !ok {
		return -1, -1
	}
	reach := (gripOff + gripShort + 4) * e.uiScale()
	row, column := placed.At(point)
	box := placed.Box
	if point.Y < box.Y-reach || point.Y > box.Y+box.H {
		row = -1
	}
	if point.X < box.X-reach || point.X > box.X+box.W {
		column = -1
	}
	if point.Y < box.Y {
		row = -1
	}
	if point.X < box.X {
		column = -1
	}
	return row, column
}

// selectCells selects cells whole: the text selection goes to the first
// cell's text, which typing writes over.
func (b *Bridge) selectCells(cells CellRange) {
	table, ok := b.tableShape(cells.Table)
	if !ok {
		b.Cells = nil
		return
	}
	rows := cells.Cells(table)
	if len(rows) == 0 || len(rows[0]) == 0 {
		b.Cells = nil
		return
	}
	anchor := table.Rows[cells.Anchor[0]][cells.Anchor[1]]
	_ = b.Controller.SelectCell(anchor)
	if cells.Anchor == cells.Head {
		b.Cells = nil
		return
	}
	b.Cells = &cells
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
	return b.Cells.Cells(table), true
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

// pasteTable writes text into the page as a table when it is one: from
// the caret's cell on in a table, or as a new table elsewhere when it has
// rows and columns. It reports whether it did.
func (b *Bridge) pasteTable(text string) bool {
	rows, ok := editor.ParseTable(text)
	if !ok {
		return false
	}
	head := b.Controller.Session().Selection().Head().Node()
	if b.Cells != nil {
		if table, ok := b.tableShape(b.Cells.Table); ok {
			top, left, _, _ := b.Cells.Bounds()
			head = table.Rows[top][left]
		}
	}
	if _, inTable := b.Controller.Table(head); inTable {
		b.Cells = nil
		_, _ = b.Controller.FillTable(head, rows)
		return true
	}
	if len(rows) < 2 || len(rows[0]) < 2 {
		return false
	}
	_, _ = b.Controller.InsertTable(rows)
	return true
}

// cellAt is the cell under a point of the element, its table's first
// cell, row and column.
func (e *Element) cellAt(x, y float32) (model.NodeID, int, int, bool) {
	if e.current == nil {
		return 0, 0, 0, false
	}
	point := e.contentPoint(x, y)
	for _, placed := range e.current.Tables() {
		if !contains(placed.Box, point) {
			continue
		}
		row, column := placed.At(point)
		return placed.Table.Node(), row, column, true
	}
	return 0, 0, 0, false
}

// dragCells follows a drag that began in a cell: into another cell of its
// table it selects the cells between whole; back in its own cell, the
// text again. It reports whether it took the move.
func (e *Element) dragCells(bridge *Bridge, x, y float32) bool {
	if e.tables.press == 0 {
		return false
	}
	table, row, column, ok := e.cellAt(x, y)
	if !ok || table != e.tables.pressTable {
		return bridge.Cells != nil
	}
	shape, ok := bridge.tableShape(table)
	if !ok {
		return false
	}
	pressRow, pressColumn, _ := shape.Find(e.tables.press)
	if pressRow == row && pressColumn == column {
		if bridge.Cells == nil {
			return false
		}
		bridge.Cells = nil
		return false
	}
	bridge.selectCells(CellRange{Table: table, Anchor: [2]int{pressRow, pressColumn}, Head: [2]int{row, column}})
	e.pointer = pointerSelecting
	return true
}

// cellsKey handles the keys a selection of whole cells takes before the
// keymap: Delete and Backspace empty them, Escape lets them go; any other
// key lets them go and goes on as it would.
func (e *Element) cellsKey(event ui.InputEvent) (bool, bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Cells == nil {
		return false, false
	}
	switch {
	case event.Key == ui.KeyBackspace || event.Key == ui.KeyDelete:
		return true, bridge.clearCells()
	case event.Key == ui.KeyEscape:
		bridge.Cells = nil
		return true, true
	case event.Mods&(ui.Cmd|ui.Ctrl) != 0:
		// Copying and the like read the cells.
		return false, false
	}
	bridge.Cells = nil
	return false, false
}

// tableMenu fills a table's menu for the cell at the caret, or the cells
// selected whole: making rows and columns beside them, aligning their
// columns, the header, and taking them away.
func (e *Element) tableMenu(m *ui.Menu, bridge *Bridge) bool {
	head := bridge.Controller.Session().Selection().Head().Node()
	table, ok := bridge.Controller.Table(head)
	if !ok {
		return false
	}
	first := table.First()
	top, left, _ := table.Find(head)
	bottom, right := top, left
	if bridge.Cells != nil && bridge.Cells.Table == first {
		top, left, bottom, right = bridge.Cells.Bounds()
	}
	m.EditItems()
	m.Separator()
	control := bridge.Controller
	if m.Item("Insert Row Above").Chosen() {
		_, _ = control.InsertTableRows(first, top, 1)
	}
	if m.Item("Insert Row Below").Chosen() {
		_, _ = control.InsertTableRows(first, bottom+1, 1)
	}
	if m.Item("Insert Column Left").Chosen() {
		_, _ = control.InsertTableColumns(first, left, 1)
	}
	if m.Item("Insert Column Right").Chosen() {
		_, _ = control.InsertTableColumns(first, right+1, 1)
	}
	m.Separator()
	align := func(to surface.CellAlign) {
		_, _ = control.ShapeTable(first, func(next *editor.Table) {
			for index := left; index <= right; index++ {
				next.Align[index] = to
			}
		})
	}
	m.Submenu("Align Column", func(m *ui.Menu) {
		current := table.Align[left]
		if m.Item("Left").Radio(current == surface.CellStart).Chosen() {
			align(surface.CellStart)
		}
		if m.Item("Center").Radio(current == surface.CellCenter).Chosen() {
			align(surface.CellCenter)
		}
		if m.Item("Right").Radio(current == surface.CellEnd).Chosen() {
			align(surface.CellEnd)
		}
	})
	if m.Item("Even Column Widths").Disabled(!uneven(table.Widths)).Chosen() {
		_, _ = control.ShapeTable(first, func(next *editor.Table) { clear(next.Widths) })
	}
	if m.Item("Header Row").Checked(table.Header).Chosen() {
		_, _ = control.ShapeTable(first, func(next *editor.Table) { next.Header = !next.Header })
	}
	m.Separator()
	rows, columns := "Delete Row", "Delete Column"
	if bottom > top {
		rows = "Delete Rows"
	}
	if right > left {
		columns = "Delete Columns"
	}
	if m.Item(rows).Chosen() {
		bridge.Cells = nil
		_, _ = control.DeleteTableRows(first, top, bottom)
	}
	if m.Item(columns).Chosen() {
		bridge.Cells = nil
		_, _ = control.DeleteTableColumns(first, left, right)
	}
	if m.Item("Delete Table").Chosen() {
		bridge.Cells = nil
		_, _ = control.DeleteTable(first)
	}
	return true
}

func uneven(widths []float32) bool {
	for _, width := range widths {
		if width > 0 {
			return true
		}
	}
	return false
}

// drawTables paints each table in view: its header's inset, the lines
// between its cells, its edge, the cells selected whole, and what a drag
// of a grip or the corner would do. Room opening between two rows or two
// columns parts the grid there, each part framed whole, so what is made
// in the room stands clear of the table's lines.
func drawTables(p *ui.Painter, rect ui.Rect, state *State) {
	visible := p.Visible()
	radius := figureRadius(state)
	line := state.Theme.Border.Default
	for _, placed := range state.Layout.Tables() {
		box := placed.Box
		box.X += rect.X
		box.Y += rect.Y + state.TopInset - state.ScrollY
		if box.Y+box.H < visible.Y || box.Y > visible.Y+visible.H {
			continue
		}
		dx, dy := box.X-placed.Box.X, box.Y-placed.Box.Y
		cell := func(row, column int) ui.Rect {
			r := placed.Cell(row, column)
			r.X += dx
			r.Y += dy
			return r
		}
		var header ui.Rect
		if placed.Table.Header && len(placed.Rows) > 0 {
			header = cell(0, 0)
			header.X, header.W = box.X, box.W
		}
		var rowGaps, columnGaps [][2]float32
		for _, gap := range append([]ui.Rect{placed.RowGap}, placed.RowsApart...) {
			rowGaps = append(rowGaps, [2]float32{gap.Y + dy, gap.H})
		}
		for _, gap := range append([]ui.Rect{placed.ColumnGap}, placed.ColumnsApart...) {
			columnGaps = append(columnGaps, [2]float32{gap.X + dx, gap.W})
		}
		ys := tableSpans(box.Y, box.H, rowGaps)
		xs := tableSpans(box.X, box.W, columnGaps)
		for _, y := range ys {
			for _, x := range xs {
				part := ui.Rect{X: x[0], Y: y[0], W: x[1] - x[0], H: y[1] - y[0]}
				p.Clip(part, radius, func() {
					if header.H > 0 {
						p.Fill(header, state.Theme.Surface.Inset, 0)
					}
					for row := 1; row < len(placed.Rows); row++ {
						if at := placed.Rows[row] + dy; at > part.Y+0.5 && at < part.Y+part.H-0.5 {
							p.Fill(ui.Rect{X: part.X, Y: at, W: part.W, H: 1}, line, 0)
						}
					}
					for column := 1; column < len(placed.Columns); column++ {
						if at := placed.Columns[column] + dx; at > part.X+0.5 && at < part.X+part.W-0.5 {
							p.Fill(ui.Rect{X: at, Y: part.Y, W: 1, H: part.H}, line, 0)
						}
					}
				})
				p.Stroke(part, line, radius, 1)
			}
		}
		drawMade(p, state, placed, cell, radius)
		if cells := state.TableCells; cells != nil && cells.Table == placed.Table.Node() {
			top, left, bottom, right := cells.Bounds()
			if bottom < len(placed.Rows) && right < len(placed.Columns) {
				from, to := cell(top, left), cell(bottom, right)
				span := ui.Rect{X: from.X, Y: from.Y, W: to.X + to.W - from.X, H: to.Y + to.H - from.Y}
				p.Clip(box, radius, func() {
					p.Fill(span, state.Theme.Prose.Selection.Alpha(state.Motion.SelectionOpacity()), 0)
					p.Stroke(span, state.Theme.Accent.Foreground.Alpha(0.85), 0, figureRingWidth)
				})
			}
		}
		drawBorder(p, state, placed, box, dx)
		drawTableEdges(p, state, placed, box)
		drawTableDrag(p, state, placed, box)
	}
}

// drawTableEdges fades the sides of a table wider than its room where
// more of it lies past them, as far as it does, up to a line's height.
func drawTableEdges(p *ui.Painter, state *State, placed textlayout.PlacedTable, box ui.Rect) {
	if placed.Overflow <= 0 {
		return
	}
	reach := state.Prose.LineHeight()
	page := state.Theme.Surface.Page
	if past := min(placed.Scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page, To: page.Alpha(0), Angle: 90}, 0)
	}
	if past := min(placed.Overflow-placed.Scroll, reach); past > 0.5 {
		p.FillGradient(ui.Rect{X: box.X + box.W - past, Y: box.Y, W: past, H: box.H}, ui.LinearGradient{From: page.Alpha(0), To: page, Angle: 90}, 0)
	}
}

// drawMade shows a row or column a create area made as the area it was,
// its tint and dashed outline fading into the cells.
func drawMade(p *ui.Painter, state *State, placed textlayout.PlacedTable, cell func(row, column int) ui.Rect, radius float32) {
	made := state.TableMade
	if made == nil || made.table != placed.Table.Node() {
		return
	}
	now := p.Now()
	deadline := made.at.Add(madeFade)
	if !now.Before(deadline) {
		return
	}
	fade := float32(1)
	if state.Animate {
		fade -= float32(now.Sub(made.at)) / float32(madeFade)
		p.AnimationFrame()
	} else {
		p.After(deadline.Sub(now))
	}
	var span ui.Rect
	switch {
	case !made.column && made.index < len(placed.Rows):
		from, to := cell(made.index, 0), cell(made.index, len(placed.Columns)-1)
		span = ui.Rect{X: from.X, Y: from.Y, W: to.X + to.W - from.X, H: from.H}
	case made.column && made.index < len(placed.Columns):
		from, to := cell(0, made.index), cell(len(placed.Rows)-1, made.index)
		span = ui.Rect{X: from.X, Y: from.Y, W: from.W, H: to.Y + to.H - from.Y}
	default:
		return
	}
	fade = components.Smoothstep(fade)
	p.Fill(span, state.Theme.Accent.Muted.Alpha(0.6*fade), radius)
	components.DashedOutline(p, span, radius, 1, state.Theme.Accent.Foreground.Alpha(fade))
}

// drawTableDrag shows the size the corner's drag would make the table,
// named on a chip.
func drawTableDrag(p *ui.Painter, state *State, placed textlayout.PlacedTable, box ui.Rect) {
	drag := state.TableDrag
	if drag == nil || drag.table != placed.Table.Node() || drag.kind != dragCorner {
		return
	}
	columns, rows := placed.Table.Columns(), len(placed.Table.Rows)
	size := ui.Rect{
		X: box.X, Y: box.Y,
		W: max(box.W/float32(columns), box.W*float32(drag.columns)/float32(columns)),
		H: max(box.H/float32(rows), box.H*float32(drag.rows)/float32(rows)),
	}
	components.DashedOutline(p, size, figureRadius(state), 1.5, state.Theme.Accent.Foreground)
	drawResizeLabel(p, size, state, fmt.Sprintf("%d × %d", drag.columns, drag.rows))
}

// tableSpans are the stretches from start over size that gaps leave, each
// gap where it opens and how far, and each stretch a start and an end:
// the whole, until a gap is open enough to part it.
func tableSpans(start, size float32, gaps [][2]float32) [][2]float32 {
	spans := [][2]float32{{start, start + size}}
	for _, gap := range gaps {
		at, open := gap[0], gap[1]
		if open < 1 {
			continue
		}
		var parted [][2]float32
		for _, span := range spans {
			if at+open <= span[0] || at >= span[1] {
				parted = append(parted, span)
				continue
			}
			if at-span[0] >= 1 {
				parted = append(parted, [2]float32{span[0], at})
			}
			if span[1]-(at+open) >= 1 {
				parted = append(parted, [2]float32{at + open, span[1]})
			}
		}
		spans = parted
	}
	return spans
}

// grown is r reaching dx further each side across and dy each side down,
// when it is there at all.
func grown(r ui.Rect, dx, dy float32) ui.Rect {
	if r.W <= 0 || r.H <= 0 {
		return r
	}
	return ui.Rect{X: r.X - dx, Y: r.Y - dy, W: r.W + 2*dx, H: r.H + 2*dy}
}

func contains(r ui.Rect, point ui.Point) bool {
	return r.W > 0 && r.H > 0 && point.X >= r.X && point.X <= r.X+r.W && point.Y >= r.Y && point.Y <= r.Y+r.H
}

func abs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

// chromeSize is the size the page's chrome is set at, and uiScale its
// share of the default.
func (e *Element) chromeSize() float32 {
	if e.UIFontSize > 0 {
		return e.UIFontSize
	}
	return theme.DefaultUIFontSize
}

func (e *Element) uiScale() float32 { return e.chromeSize() / theme.DefaultUIFontSize }

func (e *Element) pointerPoint() (ui.Point, bool) { return e.tables.pointer, e.tables.pointerIn }

// pressTable notes the cell a press begins in, for a drag across cells,
// and lets cells selected whole go, unless a secondary click is on them,
// for their menu.
func (e *Element) pressTable(event ui.InputEvent) {
	e.tables.press, e.tables.pressTable = 0, 0
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil {
		return
	}
	table, row, column, inTable := e.cellAt(event.X, event.Y)
	if event.Button != 0 && bridge.Cells != nil && inTable && bridge.Cells.Table == table {
		top, left, bottom, right := bridge.Cells.Bounds()
		if row >= top && row <= bottom && column >= left && column <= right {
			return
		}
	}
	bridge.Cells = nil
	if !inTable || event.Button != 0 {
		return
	}
	if shape, ok := bridge.tableShape(table); ok && row < len(shape.Rows) && column < len(shape.Rows[row]) {
		e.tables.press, e.tables.pressTable = shape.Rows[row][column], table
	}
}

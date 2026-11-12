package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A table's shape is changed where it is, by the controls it offers the
// page as the pointer comes by (controls.go), each a drag the page holds
// until release and then one intent; none keeps state the engine reads.
// What they are and where they stand is here; what they do as they are
// held is in table_drags.go, how they move in table_motion.go, and how
// they look in table_paint.go.
//
//   - Over a row or a column, a grip in the gutter before it or above it
//     picks its cells whole, and dragged carries the line among the others.
//     The knob at the table's far corner drags its size; a click on it
//     selects the table.
//   - A column's border lights under the pointer and drags its width; a
//     double click fits the column to its text.
//   - The pointer resting by the table's foot opens an area under it that
//     makes a row, and by its end one beside it that makes a column; in
//     the gutter by a border between two rows, or above the table by one
//     between two columns, one opens between them.
//   - A secondary click on a line's grip offers what acts on that line,
//     and on a cell the table's commands.
const (
	// tableReach is how near a table the pointer calls up its controls,
	// in DIPs at the default UI size, and tableHold how near it keeps an
	// area open; tableEdge is how far within the table's foot or end the
	// pointer calls up the area there, and tableBorder how near a border
	// between two lines, in the gutter, one between them.
	tableReach  = controlReach
	tableHold   = 56
	tableEdge   = 12
	tableBorder = 8
	// gripLong and gripShort size a grip, gripIcon its icon, gripOff sets
	// it off what it moves; cornerSize is the corner's knob, borderGrab how
	// near a column's border a press takes it, and borderRing how wide it
	// lights.
	gripLong, gripShort, gripIcon, gripOff = 22, 16, 14, 4
	cornerSize                             = 10
	borderGrab                             = 4
	borderRing                             = 2
)

// A table's controls.
const (
	partCells      part = "cells"
	partBorder     part = "border"
	partBelow      part = "below"
	partBeside     part = "beside"
	partRowArea    part = "row-area"
	partColumnArea part = "column-area"
	partCorner     part = "corner"
	partRowGrip    part = "row-grip"
	partColumnGrip part = "column-grip"
)

// areaRooms are the gap an area keeps from the table, and from the lines
// either side of it between two; the room under the foot, an area and its
// gap; and the room between two lines, an area and its gap each side.
func (el *Element) areaRooms() (gap, inner, step, between float32) {
	size, h := el.chromeSize(), el.controlMetrics().Height
	gap, inner = theme.Space06.DIPs(size), theme.Space08.DIPs(size)
	return gap, inner, h + gap, h + 2*inner
}

// tableBox is a table's box in the page.
func tableBox(u *unitLayout) ui.Rect {
	return ui.Rect{X: u.x + u.box.X, Y: u.y + u.box.Y, W: u.box.W, H: u.box.H}
}

// gripRects are a table's grips in the page: its row's before row, its
// column's above column, and its corner's.
func (el *Element) gripRects(u *unitLayout, row, column int) (ui.Rect, ui.Rect, ui.Rect) {
	t, b, scale := u.table, tableBox(u), el.chromeScale()
	long, short, off := gripLong*scale, gripShort*scale, gripOff*scale
	var rr, cr ui.Rect
	if row >= 0 && row < len(t.rows) {
		rr = ui.Rect{X: b.X - off - short, Y: u.y + t.rows[row] + (t.heights[row]-long)/2, W: short, H: long}
	}
	if column >= 0 && column < len(t.columns) {
		c := t.cell(doc.CellCoord{Column: column})
		// A column scrolled partly out of the table keeps its grip in it.
		x := max(b.X, min(u.x+c.X+(c.W-long)/2, b.X+b.W-long))
		cr = ui.Rect{X: x, Y: b.Y - off - short, W: long, H: short}
	}
	size := cornerSize * scale
	return rr, cr, ui.Rect{X: b.X + b.W - size/2, Y: b.Y + b.H - size/2, W: size, H: size}
}

// tableArea is an area as it shows over the page: its box, how far it
// shows (alpha) and its icon and label have come in, how far it slides
// in from the side, and whether it shows its icon alone.
type tableArea struct {
	part               part
	box                ui.Rect
	alpha, icon, label float32
	shift              float32
	compact            bool
}

// tableAreas are the areas a table shows, in the page, each in the room
// the layout gives it but the one beside the end, which stands in the
// margin.
func (el *Element) tableAreas(u *unitLayout) []tableArea {
	t := u.table
	gap, inner, step, between := el.areaRooms()
	var out []tableArea
	if b := t.below; b.H > 1 {
		reveal := min(1, b.H/step)
		out = append(out, tableArea{part: partBelow, alpha: reveal, icon: reveal, label: components.Smoothstep(reveal),
			box: ui.Rect{X: u.x + b.X, Y: u.y + b.Y + gap, W: b.W, H: max(1, b.H-gap)}})
	}
	if reveal := el.springs.value(tableSpring(u.pres.Unit, "beside", shareValue)); reveal > 0.001 {
		b := tableBox(u)
		out = append(out, tableArea{part: partBeside, alpha: reveal, icon: 1, label: 1, shift: (1 - reveal) * -gap, compact: true,
			box: ui.Rect{X: b.X + b.W + gap, Y: b.Y, W: el.controlMetrics().Height, H: b.H}})
	}
	if g := t.rowGap; g.H > 1 {
		reveal := min(1, g.H/between)
		out = append(out, tableArea{part: partRowArea, alpha: reveal, icon: reveal, label: components.Smoothstep(reveal),
			box: ui.Rect{X: u.x + g.X, Y: u.y + g.Y + inner, W: g.W, H: max(1, g.H-2*inner)}})
	}
	if g := t.across(t.columnGap); g.W > 1 {
		reveal := min(1, g.W/between)
		out = append(out, tableArea{part: partColumnArea, alpha: reveal, icon: reveal, label: 1, compact: true,
			box: ui.Rect{X: u.x + g.X + inner, Y: u.y + g.Y, W: max(1, g.W-2*inner), H: g.H}})
	}
	return out
}

// linesAt are the row and the column of u a page point is over, as its
// grips take them: a row across the table and its gutter, a column down it
// and the room above it.
func (el *Element) linesAt(u *unitLayout, x, y float32) (int, int) {
	t, b := u.table, tableBox(u)
	reach := (gripOff + gripShort + 4) * el.chromeScale()
	row, column := -1, -1
	if x >= b.X-reach && x < b.X+b.W {
		row = t.rowAt(y - u.y)
	}
	if y >= b.Y-reach && y < b.Y+b.H {
		column = t.columnAt(x - u.x)
	}
	return row, column
}

// controls are a table's controls, bottom to top: its cells, which offer
// the table's commands; its columns' borders; its open areas; and, while
// the pointer is by it, its corner and the grips of the lines it is over.
func (tableKind) controls(el *Element, u *unitLayout) []control {
	t := u.table
	if t == nil {
		return nil
	}
	id := u.pres.Unit
	key := func(p part, at int) controlKey { return controlKey{unit: id, part: p, at: at} }
	scale := el.chromeScale()
	b := tableBox(u)
	out := []control{{key: key(partCells, 0), rect: b, menu: func(x, y float32, at time.Time) [][]vocab.CommandID {
		el.placeForMenu(x, y, at)
		return tableKind{}.menu(el, u)
	}}}
	for c := range t.columns {
		cell := t.cell(doc.CellCoord{Column: c})
		edge := u.x + cell.X + cell.W
		if edge < b.X || edge > b.X+b.W+1 {
			continue
		}
		out = append(out, control{key: key(partBorder, c), cursor: ui.CursorResizeEW,
			rect: ui.Rect{X: edge - borderGrab*scale, Y: b.Y, W: 2 * borderGrab * scale, H: b.H},
			press: func(x, y float32, clicks int, extend bool, at time.Time) drag {
				return &borderDrag{tablePress: el.tablePress(key(partBorder, c), x, y, clicks, extend), w0: t.widths[c]}
			}})
	}
	for _, a := range el.tableAreas(u) {
		if a.alpha <= 0.5 {
			continue
		}
		out = append(out, control{key: key(a.part, 0), rect: a.box, cursor: ui.CursorPointer,
			press: func(x, y float32, clicks int, extend bool, at time.Time) drag {
				return &areaPress{tablePress: el.tablePress(key(a.part, 0), x, y, clicks, extend)}
			}})
	}
	if el.near != id {
		return out
	}
	row, column := el.gripLines(u)
	rr, cr, corner := el.gripRects(u, row, column)
	out = append(out, control{key: key(partCorner, 0), rect: grown(corner, 3*scale, 3*scale), cursor: ui.CursorResizeNWSE,
		press: func(x, y float32, clicks int, extend bool, at time.Time) drag {
			return &cornerDrag{tablePress: el.tablePress(key(partCorner, 0), x, y, clicks, extend), rows: len(t.rows), columns: len(t.columns)}
		}})
	for _, g := range []struct {
		rect ui.Rect
		part part
		at   int
		rows bool
	}{{cr, partColumnGrip, column, false}, {rr, partRowGrip, row, true}} {
		if g.rect.W <= 0 {
			continue
		}
		out = append(out, control{key: key(g.part, g.at), rect: g.rect, cursor: ui.CursorGrab,
			press: func(x, y float32, clicks int, extend bool, at time.Time) drag {
				return el.lineDrag(u, g.rows, el.tablePress(key(g.part, g.at), x, y, clicks, extend), at)
			},
			menu: func(_, _ float32, at time.Time) [][]vocab.CommandID {
				return el.pickForMenu(id, g.rows, g.at, at)
			}})
	}
	return out
}

// reach is where the pointer is by a table: the table and the gutters its
// grips and areas stand in, wider while an area of its edges is open.
// menu is a table's commands, as a cell's menu offers them.
func (tableKind) menu(*Element, *unitLayout) [][]vocab.CommandID { return tableMenu }

func (tableKind) reach(el *Element, u *unitLayout) ui.Rect {
	t := u.table
	if t == nil {
		return ui.Rect{}
	}
	scale := el.chromeScale()
	reach := tableReach * scale
	if _, open := el.tableState(u.pres.Unit).below.Open(); open {
		reach = tableHold * scale
	}
	r := grown(tableBox(u), reach, reach)
	r.W += theme.Space06.DIPs(el.chromeSize()) + el.controlMetrics().Height
	r.H += t.below.H
	return r
}

// gripLines are the row and column whose grips a table shows, -1 for none:
// those the pointer is over while it is by the table, a grip's drag holding
// its own line.
func (el *Element) gripLines(u *unitLayout) (row, column int) {
	if el.near != u.pres.Unit {
		return -1, -1
	}
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	row, column = el.linesAt(u, x, y)
	if held := el.heldTable(); held != nil && held.table == u.pres.Unit {
		switch held.part {
		case partRowGrip:
			row = held.at
		case partColumnGrip:
			column = held.at
		}
	}
	return row, column
}

// follow follows the pointer by a table, as the last layout set it out:
// the areas it calls up while the pointer is by it, each opening once the
// pointer has rested there.
func (tableKind) follow(el *Element, c *ui.Context, u *unitLayout) {
	t := u.table
	if t == nil {
		return
	}
	ts := el.tableState(u.pres.Unit)
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	scale := el.chromeScale()
	gap := theme.Space06.DIPs(el.chromeSize())
	beside := gap + el.controlMetrics().Height
	nearAt, nearOpen := ts.below.Open()
	edgeAt, rowAt, columnAt := -1, -1, -1
	rowNear, columnNear := float32(0), float32(0)
	if d, ok := el.drag.(*lineDrag); ok && d.moved {
		// A carry chooses where to land: nothing is made meanwhile.
		ts.below.Close()
		ts.rows.Close()
		ts.columns.Close()
	}
	if el.near == u.pres.Unit && el.drag == nil {
		b := tableBox(u)
		// The areas making a row and a column are called up by the
		// table's foot and its end, not from over its cells: the pointer
		// just within the edge, or past it. Where the two meet, the one
		// open holds, else the nearer edge's.
		hold := func(side int) float32 {
			if nearOpen && nearAt == side {
				return tableHold * scale
			}
			return tableReach * scale
		}
		edge := tableEdge * scale
		bottom, right := b.Y+b.H, b.X+b.W
		foot := ui.Rect{X: b.X - edge, Y: bottom - edge, W: b.W + 2*edge, H: edge + t.below.H + hold(0)}
		end := ui.Rect{X: right - edge, Y: b.Y - edge, W: edge + beside + hold(1), H: b.H + 2*edge}
		inFoot, inEnd := contains(foot, x, y), contains(end, x, y)
		switch {
		case inFoot && inEnd && nearOpen:
			edgeAt = nearAt
		case inEnd && (!inFoot || abs32(x-right) < abs32(y-bottom)):
			edgeAt = 1
		case inFoot:
			edgeAt = 0
		}
		// Room between two lines is called up from the gutter before the
		// rows, where their grips are, and above the columns, where
		// theirs are: never from over the cells, where the pointer is for
		// the text. An open room holds while the pointer is in it, the
		// border's reach around it.
		gutter, border := tableReach*scale, tableBorder*scale
		inRows := x >= b.X-gutter && x < b.X
		inColumns := y >= b.Y-gutter && y < b.Y && x >= b.X && x <= b.X+b.W
		var rowGap, columnGap ui.Rect
		if g := t.rowGap; g.H > 0 {
			rowGap = grown(ui.Rect{X: u.x + g.X - gutter, Y: u.y + g.Y, W: g.W + gutter, H: g.H}, 0, border)
		}
		if g := t.across(t.columnGap); g.W > 0 {
			columnGap = grown(ui.Rect{X: u.x + g.X, Y: u.y + g.Y - gutter, W: g.W, H: g.H + gutter}, border, 0)
		}
		for r := 1; r < len(t.rows); r++ {
			at := u.y + t.rows[r]
			if inRows && abs32(y-at) <= border || contains(rowGap, x, y) && t.rowGap.Y < t.rows[r] {
				rowAt, rowNear = r, abs32(y-at)
				break
			}
		}
		for col := 1; col < len(t.columns); col++ {
			at := u.x + t.cell(doc.CellCoord{Column: col}).X
			if inColumns && abs32(x-at) <= border || contains(columnGap, x, y) && t.columnGap.X < t.columns[col] {
				columnAt, columnNear = col, abs32(x-at)
				break
			}
		}
	}
	// One room opens between lines at a time: by a crossing of a row's
	// border and a column's, the one open already, else the nearer.
	_, rowsOpen := ts.rows.Open()
	_, columnsOpen := ts.columns.Open()
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
	ts.below.Follow(c, edgeAt)
	ts.rows.Follow(c, rowAt)
	ts.columns.Follow(c, columnAt)
	// An open area holds while the pointer is on it.
	if !el.pointerIn {
		return
	}
	for _, a := range el.tableAreas(u) {
		if !contains(grown(a.box, tableBorder*scale, tableBorder*scale), x, y) {
			continue
		}
		switch a.part {
		case partBelow, partBeside:
			ts.below.HoldOver(true)
		case partRowArea:
			ts.rows.HoldOver(true)
		case partColumnArea:
			ts.columns.HoldOver(true)
		}
	}
}

// placeForMenu puts the caret in the cell a secondary click is on, unless
// the selection holds it already, so the table's commands act there.
func (el *Element) placeForMenu(x, y float32, at time.Time) {
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	h, ok := pl.hit(x, y)
	if !ok {
		return
	}
	c := h.Stop
	r := el.Editor.Engine.Snapshot()
	if _, _, ok := vocab.Locate(r, c.Pos.Node); !ok {
		return
	}
	sel := el.Editor.Selection()
	for _, piece := range vocab.SelectionExtent(r, sel) {
		if doc.Compare(r, piece.From, c) <= 0 && doc.Compare(r, c, piece.To) <= 0 {
			return
		}
	}
	el.Editor.Do(vocab.Place{Stop: c}, at)
}

// pickForMenu picks a line whole for its grip's menu, unless a pick of
// lines holds it already, and gives that line's commands.
func (el *Element) pickForMenu(table doc.ID, rows bool, line int, at time.Time) [][]vocab.CommandID {
	menu := columnMenu
	if rows {
		menu = rowMenu
	}
	if sel := el.Editor.Selection(); sel.Kind == doc.SelectCells && sel.Cells.Table == table && sel.Cells.WholeRows == rows &&
		sel.Cells.WholeColumns == !rows && between(line, sel.Cells.Anchor, sel.Cells.Head, rows) {
		return menu
	}
	cc := doc.CellCoord{Row: line, Column: line}
	el.Editor.Do(vocab.Place{Cells: &doc.CellRange{Table: table, WholeRows: rows, WholeColumns: !rows, Anchor: cc, Head: cc}}, at)
	return menu
}

// between reports whether line at lies between two cells' lines.
func between(at int, a, b doc.CellCoord, rows bool) bool {
	from, to := a.Column, b.Column
	if rows {
		from, to = a.Row, b.Row
	}
	return at >= min(from, to) && at <= max(from, to)
}

// tableMenu are the commands a cell's menu offers, in its groups, and
// rowMenu and columnMenu those of a row's grip and a column's: what acts
// on that line alone.
var (
	tableMenu = [][]vocab.CommandID{
		{"table.row-above", "table.row-below", "table.column-left", "table.column-right"},
		{"table.align-start", "table.align-center", "table.align-end", "table.header"},
		{"table.delete-row", "table.delete-column", "table.delete"},
	}
	rowMenu = [][]vocab.CommandID{
		{"table.row-above", "table.row-below"},
		{"table.header"},
		{"table.delete-row"},
	}
	columnMenu = [][]vocab.CommandID{
		{"table.column-left", "table.column-right"},
		{"table.align-start", "table.align-center", "table.align-end"},
		{"table.delete-column"},
	}
)

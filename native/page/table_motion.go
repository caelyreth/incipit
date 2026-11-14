package page

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/components"
)

// How a table's controls move: each value a spring in the page's store
// (springs.go), stepped at the paint's time before the page is laid out,
// since what they open is room the layout gives as it is asked (tableIn),
// each room as far as its spring has come.
//
//   - The rooms the areas open, under the foot, between two rows and
//     between two columns, and the area beside the end, ease in and out;
//     the page holds the border a room between two rows opens about
//     (anchor), so it stays about the pointer.
//   - Every line a table gains comes in, from nothing or from the size of
//     the area that made it (tableIn.grow): a run of rows or of columns
//     whose cells are all new.
//   - A carry parts the grid where it would land and closes it where it no
//     longer would.

// tableState is what a table's controls keep besides their springs: where its
// row and column rooms stand, the border lit, the cells it had, the lines
// coming in, and the rooms a carry opens.
type tableState struct {
	// below are the areas at its foot, 0, and at its end, 1; rows and
	// columns those between two lines, by the line after them.
	below, rows, columns components.CreateGap
	// landing is a carried line landing, and made a line an area made.
	landing *tableLanding
	made    *tableMade

	row, column, borderAt int
	cells                 map[doc.ID]bool
	grow                  [2]lineRun
	apart                 []*apartRoom
}

// lineRun is a run of lines from to to, empty when to is not past from.
type lineRun struct{ from, to int }

// apartRoom is room a carry opens before line at, as far as target, and
// whether a frame shows in it where the carried line would land.
type apartRoom struct {
	rows   bool
	at     int
	target float32
	frame  bool
}

func (a *apartRoom) key(table doc.ID) springKey {
	name := "apart-columns"
	if a.rows {
		name = "apart-rows"
	}
	return springKey{unit: table, name: name, at: a.at, of: roomValue}
}

// apartRoom is the room a carry opens before line at.
func (ts *tableState) apartRoom(rows bool, at int) *apartRoom {
	for _, a := range ts.apart {
		if a.rows == rows && a.at == at {
			return a
		}
	}
	a := &apartRoom{rows: rows, at: at}
	ts.apart = append(ts.apart, a)
	return a
}

// tableState is the state of a table's controls, as the page remembers it
// while the table is on the page.
func (el *Element) tableState(id doc.ID) *tableState {
	return memo(el, id, "table", func() *tableState { return &tableState{row: -1, column: -1} })
}

// tableSpring is the key of one of a table's values.
func tableSpring(id doc.ID, name string, of valueKind) springKey {
	return springKey{unit: id, name: name, of: of}
}

// step moves a table's controls on to the paint's time: the rooms its
// areas and a carry open, its lines coming in, its grips and border; and
// asks its layout for what they open as they stand.
func (tableKind) step(el *Element, u *unitLayout, in *objectIn) {
	t := u.table
	if t == nil {
		return
	}
	id := u.pres.Unit
	ts := el.tableState(id)
	if m := ts.made; m != nil && el.springs.now.Sub(m.at) > madeFade {
		ts.made = nil
	}
	_, _, step, between := el.areaRooms()
	belowAt, belowOpen := ts.below.Open()
	rowsAt, rowsOpen := ts.rows.Open()
	columnsAt, columnsOpen := ts.columns.Open()
	hot, held := el.hotKey, el.heldTable()
	open := func(is bool, v float32) float32 {
		if is {
			return v
		}
		return 0
	}
	follow := func(name string, of valueKind, target float32) float32 {
		return el.springs.follow(tableSpring(id, name, of), target, chromeFollow)
	}
	below := follow("below", roomValue, open(belowOpen && belowAt == 0, step))
	follow("beside", shareValue, open(belowOpen && belowAt == 1, 1))
	if rowsOpen {
		ts.row = rowsAt
	}
	rowRoom := follow("row-room", roomValue, open(rowsOpen, between))
	if columnsOpen {
		ts.column = columnsAt
	}
	columnRoom := follow("column-room", roomValue, open(columnsOpen, between))
	mine := held != nil && held.table == id
	follow("grips", shareValue, open(el.near == id && (el.drag == nil || mine), 1))
	// A border lights under the pointer and as its drag holds it.
	if hot.unit == id && hot.part == partBorder {
		ts.borderAt = hot.at
	}
	follow("border", shareValue, open(hot.unit == id && hot.part == partBorder, 1))
	el.springs.follow(tableSpring(id, "press", shareValue), open(mine && isArea(held.part), 1), pressFollow)
	el.growLines(u, ts)
	el.stepApart(id, ts)
	tin := tableIn{}
	if below >= 0.5 {
		tin.below = below
	}
	if rowRoom >= 0.5 {
		tin.row, tin.rowRoom = ts.row, rowRoom
	}
	if columnRoom >= 0.5 {
		tin.column, tin.columnRoom = ts.column, columnRoom
	}
	k := 0
	for _, a := range ts.apart {
		if v := el.springs.value(a.key(id)); v >= 0.5 && k < maxApart {
			tin.apart[k] = lineRoom{rows: a.rows, at: a.at, room: v, left: !a.frame}
			k++
		}
	}
	for axis, g := range ts.grow {
		if g.to > g.from {
			tin.grow[axis] = lineGrow{from: g.from, to: g.to, share: el.springs.value(growKey(id, axis))}
		}
	}
	in.table = tin
}

// anchor holds the border a room between two rows opens about, while the
// pointer has it open or it closes: the page lifts by half the room, so
// what is about the pointer stays under it. A room an area made a row in
// holds nothing: the page keeps where it lifted it.
func (tableKind) anchor(el *Element, was *unitLayout, wasY float32, u *unitLayout) (float32, bool) {
	id := u.pres.Unit
	ts := el.tableState(id)
	_, opening := ts.rows.Open()
	if was.table == nil || u.table == nil || !opening && el.springs.value(tableSpring(id, "row-room", roomValue)) <= 0 {
		return 0, false
	}
	about := func(t *tableLayout) (float32, bool) {
		if ts.row <= 0 || ts.row >= len(t.rows) {
			return 0, false
		}
		y := t.rows[ts.row]
		if t.in.row == ts.row {
			y -= t.rowGap.H / 2
		}
		return y, true
	}
	from, ok := about(was.table)
	to, ok2 := about(u.table)
	if !ok || !ok2 {
		return 0, false
	}
	return u.y + to - (wasY + from), true
}

// isArea reports whether a control is one of a table's areas.
func isArea(p part) bool {
	return p == partBelow || p == partBeside || p == partRowArea || p == partColumnArea
}

func growKey(table doc.ID, axis int) springKey {
	return springKey{unit: table, name: "grow", at: axis, of: shareValue}
}

// growLines brings in the lines a table has gained since it was last laid
// out: a run of rows, or of columns, whose cells are all new, from the
// share of its size the area that made it had, else from nothing.
func (el *Element) growLines(u *unitLayout, ts *tableState) {
	t := u.table
	id := u.pres.Unit
	cells := make(map[doc.ID]bool, len(t.cells))
	fresh := [2][]bool{make([]bool, len(t.rows)), make([]bool, len(t.columns))}
	for _, f := range fresh {
		for k := range f {
			f[k] = true
		}
	}
	for i, cc := range t.cells {
		if cc.Row < 0 || i >= len(u.pres.Surfaces) {
			continue
		}
		cell := u.pres.Surfaces[i].Flow.Slot.Node
		cells[cell] = true
		if ts.cells[cell] {
			fresh[0][cc.Row], fresh[1][cc.Column] = false, false
		}
	}
	if ts.cells != nil && el.springs.animate {
		for axis, f := range fresh {
			run := lineRun{-1, -1}
			for k, is := range f {
				if is {
					if run.from < 0 {
						run.from = k
					}
					run.to = k + 1
				}
			}
			// A table wholly new comes in as a unit does.
			if run.from < 0 || run.from == 0 && run.to == len(f) {
				continue
			}
			start := float32(0)
			if m := ts.made; m != nil && m.column == (axis == 1) && m.index == run.from {
				start = m.from
			}
			ts.grow[axis] = run
			el.springs.jump(growKey(id, axis), start)
		}
	}
	ts.cells = cells
	for axis, g := range ts.grow {
		if g.to <= g.from {
			continue
		}
		key := growKey(id, axis)
		el.springs.follow(key, 1, layoutSettle)
		if el.springs.settled(key) {
			ts.grow[axis] = lineRun{}
		}
	}
}

// stepApart parts the grid where a carried line would land, once the
// pointer has stayed by the place a moment, the frame it leaves folding
// away, and lets each room close that is no longer asked for.
func (el *Element) stepApart(id doc.ID, ts *tableState) {
	if d, ok := el.drag.(*lineDrag); ok && d.moved && d.table == id {
		if d.over != d.to {
			if left := carrySettle - el.springs.now.Sub(d.since); el.springs.animate && left > 0 {
				// The page parts once the pointer has stayed: frames come
				// till then.
				el.springs.moving = true
			} else {
				d.to = d.over
			}
		}
		for _, a := range ts.apart {
			if a.frame && (a.rows != d.rows || a.at != d.to) {
				a.target = 0
			}
		}
		if d.moves(d.to) {
			a := ts.apartRoom(d.rows, d.to)
			a.target, a.frame = d.size, true
		}
	}
	kept := ts.apart[:0]
	for _, a := range ts.apart {
		v := el.springs.follow(a.key(id), a.target, carryPart)
		if a.target > 0 || v >= 0.5 {
			kept = append(kept, a)
		}
	}
	ts.apart = kept
}

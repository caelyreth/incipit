package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
)

// How a table's controls move: each value a spring in the page's store
// (controls.go), stepped at the paint's time before the page is laid out,
// since what they open is room the layout gives as it is asked (tableIn),
// each room as far as its spring has come.
//
//   - The rooms the areas open, under the foot, between two rows and
//     between two columns, and the area beside the end, ease in and out;
//     the page lifts by half the room between two rows, so the border stays
//     about the pointer.
//   - Every line a table gains comes in, from nothing or from the size of
//     the area that made it (tableIn.grow): a run of rows or of columns
//     whose cells are all new.
//   - A carry parts the grid where it would land and closes it where it no
//     longer would.

// tableState is what a table's motion keeps besides its springs: where its
// row and column rooms stand, the border lit, the cells it had, the lines
// coming in, the rooms a carry opens, and what its layout is asked as they
// stand.
type tableState struct {
	row, column, borderAt int
	cells                 map[doc.ID]bool
	grow                  [2]lineRun
	apart                 []*apartRoom
	asked                 tableIn
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
	return springKey{unit: table, name: name, at: a.at}
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

// stepTables moves each table's controls on to now: the rooms its areas
// and a carry open, its lines coming in, its grips and border, and sets
// what its layout is asked as they stand; the page lifts by half the room
// open between two rows.
func (el *Element) stepTables(now time.Time, animate bool) {
	pl := el.Editor.layout
	ch := &el.chrome
	if pl == nil {
		return
	}
	if ch.tables == nil {
		ch.tables = map[doc.ID]*tableState{}
	}
	if m := ch.made; m != nil && now.Sub(m.at) > madeFade {
		ch.made = nil
	}
	_, _, step, between := el.areaRooms()
	belowAt, belowOpen := ch.below.Open()
	rowsAt, rowsOpen := ch.rows.Open()
	columnsAt, columnsOpen := ch.columns.Open()
	hot, held := el.hotKey, el.heldTable()
	open := func(is bool, v float32) float32 {
		if is {
			return v
		}
		return 0
	}
	lift := float32(0)
	seen := map[doc.ID]bool{}
	for i, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		id := u.pres.Unit
		seen[id] = true
		ts := ch.tables[id]
		if ts == nil {
			ts = &tableState{row: -1, column: -1}
			ch.tables[id] = ts
		}
		follow := func(name string, target float32, unit bool) float32 {
			return el.springs.follow(springKey{unit: id, name: name}, target, chromeFollow, unit)
		}
		below := follow("below", open(belowOpen && belowAt == 2*i, step), false)
		follow("beside", open(belowOpen && belowAt == 2*i+1, 1), true)
		if rowsOpen && rowsAt/gapStride == i {
			ts.row = rowsAt % gapStride
		}
		rowRoom := follow("row-room", open(rowsOpen && rowsAt/gapStride == i, between), false)
		if columnsOpen && columnsAt/gapStride == i {
			ts.column = columnsAt % gapStride
		}
		columnRoom := follow("column-room", open(columnsOpen && columnsAt/gapStride == i, between), false)
		mine := held != nil && held.table == id
		follow("grips", open(ch.near == id && (ch.inside || mine) && (el.drag == nil || mine), 1), true)
		// A border lights under the pointer and as its drag holds it.
		if hot.unit == id && hot.part == partBorder {
			ts.borderAt = hot.at
		}
		follow("border", open(hot.unit == id && hot.part == partBorder, 1), true)
		el.springs.follow(springKey{unit: id, name: "press"}, open(mine && isArea(held.part), 1), pressFollow, true)
		el.growLines(u, ts, now, animate)
		el.stepApart(id, ts, now, animate)
		if ts.row > 0 && ts.row < len(t.rows) {
			lift += rowRoom / 2
		}
		in := tableIn{}
		if below >= 0.5 {
			in.below = below
		}
		if rowRoom >= 0.5 {
			in.row, in.rowRoom = ts.row, rowRoom
		}
		if columnRoom >= 0.5 {
			in.column, in.columnRoom = ts.column, columnRoom
		}
		k := 0
		for _, a := range ts.apart {
			if v := el.springs.value(a.key(id)); v >= 0.5 && k < maxApart {
				in.apart[k] = lineRoom{rows: a.rows, at: a.at, room: v}
				k++
			}
		}
		for axis, g := range ts.grow {
			if g.to > g.from {
				in.grow[axis] = lineGrow{from: g.from, to: g.to, share: el.springs.value(growKey(id, axis))}
			}
		}
		ts.asked = in
	}
	for id := range ch.tables {
		if !seen[id] {
			delete(ch.tables, id)
		}
	}
	// What is about the pointer stays under it as room opens between two
	// rows, a glide under way carried along.
	if delta := lift - ch.lift; delta != 0 {
		ch.lift = lift
		el.scroll += delta
		el.glide.Shift(delta)
	}
}

// isArea reports whether a control is one of a table's areas.
func isArea(p part) bool {
	return p == partBelow || p == partBeside || p == partRowArea || p == partColumnArea
}

func growKey(table doc.ID, axis int) springKey {
	return springKey{unit: table, name: "grow", at: axis}
}

// growLines brings in the lines a table has gained since it was last laid
// out: a run of rows, or of columns, whose cells are all new, from the
// share of its size the area that made it had, else from nothing.
func (el *Element) growLines(u *unitLayout, ts *tableState, now time.Time, animate bool) {
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
	if ts.cells != nil && animate {
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
			if m := el.chrome.made; m != nil && m.table == id && m.column == (axis == 1) && m.index == run.from {
				start = m.from
			}
			ts.grow[axis] = run
			el.springs.jump(growKey(id, axis), start, true)
		}
	}
	ts.cells = cells
	for axis, g := range ts.grow {
		if g.to <= g.from {
			continue
		}
		key := growKey(id, axis)
		el.springs.follow(key, 1, figureFade, true)
		if el.springs.settled(key) {
			ts.grow[axis] = lineRun{}
		}
	}
}

// stepApart parts the grid where a carried line would land, once the
// pointer has stayed by the place a moment, the frame it leaves folding
// away, and lets each room close that is no longer asked for.
func (el *Element) stepApart(id doc.ID, ts *tableState, now time.Time, animate bool) {
	if d, ok := el.drag.(*lineDrag); ok && d.moved && d.table == id {
		if d.over != d.to {
			if left := carrySettle - now.Sub(d.since); animate && left > 0 {
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
		v := el.springs.follow(a.key(id), a.target, carryPart, false)
		if a.target > 0 || v >= 0.5 {
			kept = append(kept, a)
		}
	}
	ts.apart = kept
}

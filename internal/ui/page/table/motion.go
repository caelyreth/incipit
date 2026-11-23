package table

import (
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/ui/components"
)

// How a table's controls move: each value a spring in the page's store
// (springs.go), stepped at the paint's time before the page is laid out,
// since what they open is room the layout gives as it is asked (In),
// each room as far as its spring has come.
//
//   - The rooms the areas open, under the foot, between two rows and
//     between two columns, and the area beside the end, ease in and out;
//     the page holds the border a room between two rows opens about
//     (anchor), so it stays about the pointer.
//   - Its rows and its columns are each a flow (flows.go), a line known by
//     its cells across layouts: a line gained comes in, from nothing or
//     from the size of the area that made it; one lost goes, drawn as it
//     was while its room closes; and lines reordered glide to their
//     places, one carried landing from where it was let go (In.lines).
//   - A carry parts the grid where it would land and closes it where it no
//     longer would.

// State is what a table's controls keep besides their springs: where its
// row and column rooms stand, the border lit, the cells it had, the lines
// coming in, and the rooms a carry opens.
type State struct {
	// below are the areas at its foot, 0, and at its end, 1; rows and
	// columns those between two lines, by the line after them.
	below, rows, columns components.CreateGap
	// made is a line an area made.
	made *tableMade

	row, column, borderAt int
	apart                 []*apartRoom
	// lines are its rows' and its columns' flows; lineOf each cell's line,
	// by the identity its flow knows it by, and ids the lines in order, as
	// they were last told; wide each column's width as last laid out
	// whole.
	lines  [2]Flow
	lineOf [2]map[doc.ID]doc.ID
	ids    [2][]doc.ID
	wide   map[doc.ID]float32
}

// NewState returns an initialized table control state.
func NewState() *State { return &State{row: -1, column: -1} }

// MadeInfo reports the most recent line created via an area, if any.
func (s *State) MadeInfo() (index int, column bool, from float32, ok bool) {
	if s == nil || s.made == nil {
		return 0, false, 0, false
	}
	return s.made.index, s.made.column, s.made.from, true
}

// apartRoom is room a carry opens before line at, as far as target, and
// whether a frame shows in it where the carried line would land.
type apartRoom struct {
	rows   bool
	at     int
	target float32
	frame  bool
}

func (a *apartRoom) key(table doc.ID) SpringKey {
	name := "apart-columns"
	if a.rows {
		name = "apart-rows"
	}
	return SpringKey{Unit: table, Name: name, At: a.at, Of: RoomValue}
}

// apartRoom is the room a carry opens before line at.
func (ts *State) apartRoom(rows bool, at int) *apartRoom {
	for _, a := range ts.apart {
		if a.rows == rows && a.at == at {
			return a
		}
	}
	a := &apartRoom{rows: rows, at: at}
	ts.apart = append(ts.apart, a)
	return a
}

// springKey is the key of one of a table's values.
func springKey(id doc.ID, name string, of ValueKind) SpringKey {
	return SpringKey{Unit: id, Name: name, Of: of}
}

// Step moves a table's controls on to the paint's time: the rooms its
// areas and a carry open, its lines coming in, its grips and border; and
// returns what they open as they stand.
func stateFlow(h Host, ts *State, axis int) Flow {
	if ts.lines[axis] == nil {
		ts.lines[axis] = h.NewFlow()
	}
	return ts.lines[axis]
}

func (Kind) Step(h Host, u *Unit) In {
	t := u.Table
	if t == nil {
		return In{}
	}
	id := u.Pres.Unit
	ts := h.State(id, true)
	if m := ts.made; m != nil && h.Now().Sub(m.at) > madeFade {
		ts.made = nil
	}
	_, _, step, between := areaRooms(h)
	belowAt, belowOpen := ts.below.Open()
	rowsAt, rowsOpen := ts.rows.Open()
	columnsAt, columnsOpen := ts.columns.Open()
	hot, held := h.HotKey(), heldTable(h)
	open := func(is bool, v float32) float32 {
		if is {
			return v
		}
		return 0
	}
	follow := func(name string, of ValueKind, target float32) float32 {
		return h.SpringFollow(springKey(id, name, of), target, chromeFollow)
	}
	below := follow("below", RoomValue, open(belowOpen && belowAt == 0, step))
	follow("beside", ShareValue, open(belowOpen && belowAt == 1, 1))
	if rowsOpen {
		ts.row = rowsAt
	}
	rowRoom := follow("row-room", RoomValue, open(rowsOpen, between))
	if columnsOpen {
		ts.column = columnsAt
	}
	columnRoom := follow("column-room", RoomValue, open(columnsOpen, between))
	mine := held != nil && held.table == id
	follow("grips", ShareValue, open(h.Near() == id && (!h.HasDrag() || mine), 1))
	// A border lights under the pointer and as its Drag holds it.
	if hot.Unit == id && hot.Part == partBorder {
		ts.borderAt = hot.At
	}
	follow("border", ShareValue, open(hot.Unit == id && hot.Part == partBorder, 1))
	h.SpringFollow(springKey(id, "press", ShareValue), open(mine && isArea(held.Part), 1), pressFollow)
	lines := flowLines(h, u, ts)
	stepApart(h, id, ts)
	tin := In{}
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
		if v := h.SpringValue(a.key(id)); v >= 0.5 && k < maxApart {
			tin.apart[k] = lineRoom{rows: a.rows, at: a.at, room: v}
			k++
		}
	}
	tin.lines = lines
	return tin
}

// anchor holds the border a room between two rows opens about, while the
// pointer has it open or it closes: the page lifts by half the room, so
// what is about the pointer stays under it. A room an area made a row in
// holds nothing: the page keeps where it lifted it.
func (Kind) Anchor(h Host, was *Unit, wasY float32, u *Unit) (float32, bool) {
	id := u.Pres.Unit
	ts := h.State(id, true)
	_, opening := ts.rows.Open()
	if was.Table == nil || u.Table == nil || !opening && h.SpringValue(springKey(id, "row-room", RoomValue)) <= 0 {
		return 0, false
	}
	about := func(t *Layout) (float32, bool) {
		if ts.row <= 0 || ts.row >= len(t.rows) {
			return 0, false
		}
		y := t.rows[ts.row]
		if t.in.row == ts.row {
			y -= t.rowGap.H / 2
		}
		return y, true
	}
	from, ok := about(was.Table)
	to, ok2 := about(u.Table)
	if !ok || !ok2 {
		return 0, false
	}
	return u.Y + to - (wasY + from), true
}

// isArea reports whether a control is one of a table's areas.
func isArea(p Part) bool {
	return p == partBelow || p == partBeside || p == partRowArea || p == partColumnArea
}

// flowLines tells a table's rows and columns, as its last layout has them,
// to their flows, steps them, and gives how they are drawn on their way:
// nil when they stand. A line is known by the identity its cells had, so a
// line moved, or its cells' text changed, is the line it was; one whose
// cells are all new is new, entering from the area that made it, where
// one did.
func flowLines(h Host, u *Unit, ts *State) *lineMotion {
	t := u.Table
	now, animate := h.Now(), h.Animate()
	n := [2]int{len(t.rows), len(t.columns)}
	var cells [2][][]doc.ID
	for axis := range cells {
		cells[axis] = make([][]doc.ID, n[axis])
	}
	for i, cc := range t.cells {
		if cc.Row < 0 || i >= len(u.Pres.Surfaces) {
			continue
		}
		c := u.Pres.Surfaces[i].Flow.Slot.Node
		cells[0][cc.Row] = append(cells[0][cc.Row], c)
		cells[1][cc.Column] = append(cells[1][cc.Column], c)
	}
	if ts.wide == nil {
		ts.wide = map[doc.ID]float32{}
	}
	lm := &lineMotion{}
	moving, any := false, false
	for axis := range 2 {
		ids := make([]doc.ID, n[axis])
		of := map[doc.ID]doc.ID{}
		taken := map[doc.ID]bool{}
		for k, cs := range cells[axis] {
			for _, c := range cs {
				if was, ok := ts.lineOf[axis][c]; ok && !taken[was] {
					ids[k] = was
					break
				}
			}
			for _, c := range cs {
				if ids[k] != 0 {
					break
				}
				if !taken[c] {
					ids[k] = c
				}
			}
			taken[ids[k]] = true
			for _, c := range cs {
				of[c] = ids[k]
			}
		}
		f := stateFlow(h, ts, axis)
		same := f.Holds(ids)
		in := make([]FlowIn, n[axis])
		at := float32(0)
		for k, id := range ids {
			extent, drawn := float32(0), float32(0)
			if axis == 0 {
				extent, drawn = t.natural[k], t.rows[k]
			} else {
				drawn = t.columns[k]
				if _, asked := t.in.lines.extentOf(1, k); !asked || !f.Knows(id) {
					ts.wide[id] = t.widths[k]
				}
				extent = ts.wide[id]
			}
			if same {
				// As it stands, it was drawn there.
				f.Saw(id, drawn)
			}
			in[k] = FlowIn{ID: id, Place: at, Extent: extent, Moves: true, Kept: keptLine{u: u, index: k}}
			if m := ts.made; m != nil && m.column == (axis == 1) && m.index == k && !f.Knows(id) {
				in[k].From = m.from * extent
			}
			at += extent
		}
		f.Tell(in, now, animate)
		ts.lineOf[axis], ts.ids[axis] = of, ids
		moving = f.Step(now, animate) || moving
		lm.extent[axis] = make([]float32, n[axis])
		lm.offset[axis] = make([]float32, n[axis])
		index := make(map[doc.ID]int, n[axis])
		for k, id := range ids {
			index[id] = k
			e, _, o := f.Drawn(id, in[k].Extent)
			lm.extent[axis][k], lm.offset[axis][k] = -1, o
			if e != in[k].Extent {
				lm.extent[axis][k] = e
				if axis == 1 {
					lm.extent[axis][k] = e / max(in[k].Extent, 1)
				}
				any = true
			}
			any = any || o != 0
		}
		for _, x := range f.Exits() {
			after, cover := -1, -1
			if k, ok := index[x.After]; ok {
				after = k
			} else if x.After != 0 {
				continue
			}
			if x.Cover != 0 {
				k, ok := index[x.Cover]
				if !ok {
					continue
				}
				cover = k
			}
			lm.gone[axis] = append(lm.gone[axis], lineGone{after: after, cover: cover, room: x.Extent, lead: x.Lead,
				alpha: x.Alpha, kept: x.Kept.(keptLine)})
			any = true
		}
	}
	if moving {
		h.MarkMoving()
	}
	if !any {
		return nil
	}
	return lm
}

// stepAPart parts the grid where a carried line would land, once the
// pointer has stayed by the place a moment, the frame it leaves folding
// away, and lets each room close that is no longer asked for.
func stepApart(h Host, id doc.ID, ts *State) {
	if d, ok := h.ActiveDrag().(*lineDrag); ok && d.moved && d.table == id {
		if d.over != d.to {
			if left := carrySettle - h.Now().Sub(d.since); h.Animate() && left > 0 {
				// The page parts once the pointer has stayed: frames come
				// till then.
				h.MarkMoving()
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
		v := h.SpringFollow(a.key(id), a.target, carryPart)
		if a.target > 0 || v >= 0.5 {
			kept = append(kept, a)
		}
	}
	ts.apart = kept
}

package page

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// What a table's controls do as they are held, each the page's drag until
// release and then one intent:
//
//   - A line's grip picks the line's cells whole on a click (a Place
//     carrying the cell range; Shift stretches a pick of the same lines).
//     Dragged, it carries a copy of the line under the pointer, the line
//     veiled in its place; where the copy's middle stands among the lines'
//     middles as the drag began is where it would land, and once the
//     pointer has stayed there carrySettle the grid parts there, a frame
//     its size opening as the one it leaves closes. Letting go moves it
//     there (EditGrid), the frame going at once as the line stands in it,
//     the room it left opening at once and closing behind it, and the copy
//     landing in its place, as it does when Escape sets it back.
//   - The corner's drag sets the table's size by whole lines, kept on
//     letting go (ResizeTable); a click on it selects the table.
//   - A border's drag sets its column's width, previewed in the layout,
//     and a double click fits the column to its text (ShapeColumn).
//   - A click on an area makes the line it stands for where it stands
//     (EditGrid): its room goes at once and the line comes in from its
//     size, the page keeping where the room lifted it.
const (
	// dragSlop is how far a press goes before it is a drag.
	dragSlop = 3
	// carryVeil is how far a carried line is hidden in its place.
	carryVeil = 0.6
	// madeFade is how long a made line shows the area it was, and
	// carrySettle how long the pointer stays by a place before the grid
	// parts there.
	madeFade    = 2 * components.MotionNormal
	carrySettle = 120 * time.Millisecond
)

// tablePress is what every press on a table's control keeps: the control,
// the press, where it began and where the pointer is, in the page, and
// whether it has gone far enough to be a drag.
type tablePress struct {
	el           *Element
	table        doc.ID
	part         part
	at           int
	clicks       int
	extend       bool
	x0, y0, x, y float32
	moved        bool
}

func (el *Element) tablePress(k controlKey, x, y float32, clicks int, extend bool) tablePress {
	return tablePress{el: el, table: k.unit, part: k.part, at: k.at, clicks: clicks, extend: extend, x0: x, y0: y, x: x, y: y}
}

func (p *tablePress) key() controlKey      { return controlKey{unit: p.table, part: p.part, at: p.at} }
func (p *tablePress) pressOf() *tablePress { return p }

// follow takes the pointer to x, y and reports whether the press is a
// drag.
func (p *tablePress) follow(x, y float32) bool {
	p.x, p.y = x, y
	if !p.moved && max(abs32(x-p.x0), abs32(y-p.y0)) >= dragSlop {
		p.moved = true
	}
	return p.moved
}

// unit is the table as the last layout set it out.
func (p *tablePress) unit() *unitLayout {
	u := p.el.Editor.layout.unitOf(p.table)
	if u == nil || u.table == nil {
		return nil
	}
	return u
}

// heldTable is the press on a table's control the page holds, if any.
func (el *Element) heldTable() *tablePress {
	if d, ok := el.drag.(interface{ pressOf() *tablePress }); ok {
		return d.pressOf()
	}
	return nil
}

// lineDrag is a press on a line's grip: a click picks the line, a drag
// carries it. start is the line's box as it began, in the unit, mids the
// middles of the lines then and size its size, along the way it moves;
// over is the place the pointer is by since since, and to the place the
// grid parts at, each before the line of its number.
type lineDrag struct {
	tablePress
	rows  bool
	start ui.Rect
	mids  []float32
	size  float32
	over  int
	since time.Time
	to    int
}

func (el *Element) lineDrag(u *unitLayout, rows bool, p tablePress, at time.Time) *lineDrag {
	t := u.table
	d := &lineDrag{tablePress: p, rows: rows, start: t.line(u, rows, p.at), over: p.at, to: p.at, since: at}
	n := len(t.columns)
	if rows {
		n = len(t.rows)
	}
	for i := range n {
		r := t.line(u, rows, i)
		if rows {
			d.mids = append(d.mids, r.Y+r.H/2)
		} else {
			d.mids = append(d.mids, r.X+r.W/2)
		}
	}
	d.size = d.start.W
	if rows {
		d.size = d.start.H
	}
	return d
}

func (d *lineDrag) cursor() ui.Cursor {
	if d.moved {
		return ui.CursorGrabbing
	}
	return ui.CursorGrab
}

// corner is where the carried copy's corner is, in the unit.
func (d *lineDrag) corner() ui.Point {
	if d.rows {
		return ui.Point{X: d.start.X, Y: d.start.Y + d.y - d.y0}
	}
	return ui.Point{X: d.start.X + d.x - d.x0, Y: d.start.Y}
}

// moves reports whether landing before line slot moves the carried line:
// either side of its own place is where it is.
func (d *lineDrag) moves(slot int) bool { return slot != d.tablePress.at && slot != d.tablePress.at+1 }

// move carries the copy, and notes the place it is by: by its middle
// among the lines' middles as they stood as it began.
func (d *lineDrag) move(x, y float32, at time.Time) {
	if !d.follow(x, y) {
		return
	}
	corner := d.corner()
	middle := corner.X + d.size/2
	if d.rows {
		middle = corner.Y + d.size/2
	}
	over := 0
	for _, mid := range d.mids {
		if mid < middle {
			over++
		}
	}
	if over != d.over {
		d.over, d.since = over, at
	}
}

// finish picks the line on a click, or lets the carried line go.
func (d *lineDrag) finish(canceled bool, at time.Time) {
	switch {
	case d.moved:
		d.drop(canceled, at)
	case !canceled:
		axis := profile.Rows
		if !d.rows {
			axis = profile.Columns
		}
		cr := doc.CellRange{Table: d.table, WholeRows: axis == profile.Rows, WholeColumns: axis == profile.Columns}
		line := doc.CellCoord{Row: d.tablePress.at, Column: d.tablePress.at}
		cr.Anchor, cr.Head = line, line
		if sel := d.el.Editor.Selection(); d.extend && sel.Kind == doc.SelectCells && sel.Cells.Table == d.table &&
			sel.Cells.WholeRows == cr.WholeRows && sel.Cells.WholeColumns == cr.WholeColumns {
			cr.Anchor = sel.Cells.Anchor
		}
		d.el.Editor.Do(vocab.Place{Cells: &cr}, at)
	}
}

// drop lets a carried line go: where the pointer is, in one step of
// history, unless canceled or it would land where it is, even where the
// pointer let go before the grid parted for it. It lands from where it
// was let go, as its line's flow has it; the frame it would land in goes
// at once, as the line stands there, and the lines it left close behind
// it.
func (d *lineDrag) drop(canceled bool, at time.Time) {
	el := d.el
	from := d.tablePress.at
	d.to = d.over
	ts := el.tableState(d.table)
	axis, corner := 1, d.corner().X
	if d.rows {
		axis, corner = 0, d.corner().Y
	}
	if from < len(ts.ids[axis]) {
		ts.lines[axis].from(ts.ids[axis][from], corner, el.springs.now)
	}
	for _, a := range ts.apart {
		if a.frame && a.target > 0 {
			a.target = 0
			el.springs.jump(a.key(d.table), 0)
		}
		a.frame = false
	}
	if canceled || !d.moves(d.to) {
		return
	}
	grid := profile.Columns
	if d.rows {
		grid = profile.Rows
	}
	to := carryIndex(from, d.to)
	el.Editor.Do(profile.EditGrid{Op: profile.GridMove, Axis: grid, Table: d.table, At: from, Count: 1, To: to}, at)
}

// carryIndex is where a line from goes, landing before slot.
func carryIndex(from, slot int) int {
	if slot > from {
		return slot - 1
	}
	return slot
}

// cornerDrag is a press on a table's corner: a click selects the table, a
// drag sets its size by whole lines.
type cornerDrag struct {
	tablePress
	rows, columns int
}

func (d *cornerDrag) cursor() ui.Cursor { return ui.CursorResizeNWSE }

func (d *cornerDrag) move(x, y float32, _ time.Time) {
	u := d.unit()
	if !d.follow(x, y) || u == nil {
		return
	}
	t := u.table
	rows, columns := len(t.rows), len(t.columns)
	cellW := t.columns[columns-1] + t.widths[columns-1]
	cellH := t.rows[rows-1] + t.heights[rows-1]
	d.columns = max(1, columns+int(math.Round(float64((x-d.x0)/(cellW/float32(columns))))))
	d.rows = max(1, rows+int(math.Round(float64((y-d.y0)/(cellH/float32(rows))))))
}

func (d *cornerDrag) finish(canceled bool, at time.Time) {
	u := d.unit()
	switch {
	case canceled || u == nil:
	case !d.moved:
		d.el.Editor.Do(vocab.Place{Node: d.table}, at)
	case d.rows != len(u.table.rows) || d.columns != len(u.table.columns):
		d.el.Editor.Do(profile.ResizeTable{Table: d.table, Rows: d.rows, Columns: d.columns}, at)
	}
}

// borderDrag is a press on a column's border: a drag sets the column's
// width, from w0, and a double click fits it to its text.
type borderDrag struct {
	tablePress
	w0 float32
}

func (d *borderDrag) cursor() ui.Cursor { return ui.CursorResizeEW }

func (d *borderDrag) move(x, y float32, _ time.Time) { d.follow(x, y) }

func (d *borderDrag) width() float32 { return max(d.w0+d.x-d.x0, leastColumn) }

// ask previews the width in the table's layout.
func (d *borderDrag) ask(asks map[doc.ID]objectIn) {
	if !d.moved {
		return
	}
	a := askOf(asks, d.table)
	a.table.sized, a.table.width = d.tablePress.at, d.width()
	asks[d.table] = a
}

func (d *borderDrag) finish(canceled bool, at time.Time) {
	u := d.unit()
	if canceled || u == nil {
		return
	}
	var col profile.Column
	if shape := profile.TableShapeOf(u.pres.Node); d.tablePress.at < len(shape.Columns) {
		col = shape.Columns[d.tablePress.at]
	}
	switch {
	case d.clicks >= 2 && !d.moved:
		// A double click fits the column to its text, or lets it go.
		col.Fit, col.Width = !col.Fit, 0
	case d.moved:
		col.Width, col.Fit = int(math.Round(float64(d.width()))), false
	default:
		return
	}
	d.el.Editor.Do(profile.ShapeColumn{Table: d.table, Column: d.tablePress.at, Shape: col}, at)
}

// areaPress is a press on an area: it is clicked, not dragged.
type areaPress struct {
	tablePress
}

func (d *areaPress) cursor() ui.Cursor              { return ui.CursorPointer }
func (d *areaPress) move(x, y float32, _ time.Time) { d.x, d.y = x, y }

func (d *areaPress) finish(canceled bool, at time.Time) {
	if u := d.unit(); !canceled && u != nil {
		d.el.useArea(u, d.part, at)
	}
}

// tableMade is a line an area made: it comes in from the area's size, as
// a share of its own, the area's outline fading into it.
type tableMade struct {
	column bool
	index  int
	from   float32
	at     time.Time
}

// useArea makes the line an area makes, where it stands: the area's room
// goes at once, the line coming in from its size, and the page keeps where
// the room lifted it.
func (el *Element) useArea(u *unitLayout, p part, at time.Time) {
	t, id := u.table, u.pres.Unit
	ts := el.tableState(id)
	rows, columns := len(t.rows), len(t.columns)
	rowShare := func(size float32) float32 { return min(1, size/max(1, t.heights[rows-1])) }
	room := func(name string) springKey { return tableSpring(id, name, roomValue) }
	axis, index, share := profile.Rows, rows, float32(0)
	switch p {
	case partBelow:
		ts.below.Use()
		index, share = rows, rowShare(t.below.H)
		el.springs.jump(room("below"), 0)
	case partBeside:
		ts.below.Use()
		axis, index = profile.Columns, columns
		el.springs.jump(tableSpring(id, "beside", shareValue), 0)
	case partRowArea:
		ts.rows.Use()
		index, share = ts.row, rowShare(t.rowGap.H)
		el.springs.jump(room("row-room"), 0)
	case partColumnArea:
		ts.columns.Use()
		axis, index = profile.Columns, ts.column
		share = min(1, t.columnGap.W*float32(columns+1)/max(1, u.box.W))
		el.springs.jump(room("column-room"), 0)
	}
	n := rows
	if axis == profile.Columns {
		n = columns
	}
	ts.made = &tableMade{column: axis == profile.Columns, index: index, from: share, at: at}
	in := profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: id, At: index, Count: 1, Before: true}
	if index >= n {
		in = profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: id, At: n - 1, Count: 1}
	}
	el.Editor.Do(in, at)
}

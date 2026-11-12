package page

import (
	"fmt"
	"math"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A table's shape is changed where it is, by the controls the page shows
// around it as the pointer comes by, each a gesture the page holds until
// release and then one intent; none keeps state the engine reads.
//
//   - Over a row or a column, a grip in the gutter before it or above it
//     picks its cells whole (a Place carrying the cell range); dragged, it
//     carries the line among the others, which make room for it, and
//     letting go moves it there (EditGrid). The knob at the table's far
//     corner drags its size, outlined and named as it goes and kept on
//     letting go (ResizeTable); a click on it selects the table. Escape
//     drops any drag.
//   - A column's border drags its width, previewed as the layout sets it,
//     and a double click on it fits the column to its text (ShapeColumn).
//   - The pointer resting by the table's foot opens a dashed area there
//     that makes a row, and by its end one that makes a column; in the
//     gutter by a border between two rows, or above the table by one
//     between two columns, an area opens between them, the lines parting
//     around it. An area is room the layout gives as it is asked
//     (tableIn), so the unit springs to it as to any room (rooms.go), and
//     what it makes comes in where it stood, its outline fading into it.
//   - A secondary click on a cell opens the table's commands, each through
//     the command table.
const (
	// tableReach is how near a table the pointer calls up its controls,
	// in DIPs at the default UI size, and tableHold how near it keeps an
	// area open; tableEdge is how far within the table's foot or end the
	// pointer calls up the area there, and tableBorder how near a border
	// between two lines, in the gutter, one between them.
	tableReach  = 28
	tableHold   = 56
	tableEdge   = 12
	tableBorder = 8
	// gripLong and gripShort size a grip, gripIcon its icon, gripOff sets
	// it off what it moves; cornerSize is the corner's knob, and
	// borderGrab how near a column's border a press takes it.
	gripLong, gripShort, gripIcon, gripOff = 22, 16, 14, 4
	cornerSize                             = 10
	borderGrab                             = 4
	// dragSlop is how far a press goes before it is a drag.
	dragSlop = 3
	// gapStride numbers an area by its table and its place.
	gapStride = 1 << 12
)

// madeFade is how long a made row or column shows the area it was.
const madeFade = 2 * components.MotionNormal

// control is what of a table's controls a place is on.
type control uint8

const (
	controlNone control = iota
	controlRow
	controlColumn
	controlCorner
	controlBorder
	controlArea
)

// chromeHit is a table's control under a page point: of which table, and
// its line, a grip's row or column or a border's column.
type chromeHit struct {
	u    *unitLayout
	kind control
	at   int
}

// tableChrome is what the page keeps of its tables' controls across
// frames.
type tableChrome struct {
	// near is the table the pointer is by, and row and column the lines
	// it is over, -1 for none; inside is whether the pointer is on the
	// page at all.
	near        doc.ID
	row, column int
	inside      bool
	// grips are how far each table's grips have come in.
	grips map[doc.ID]*motion.Spring
	// rows and columns are the areas between and after a table's rows
	// and its columns, numbered by gapStride.
	rows, columns components.CreateGap
	drag          *tableDrag
	made          *tableMade
}

// tableDrag is a press on a table's control, until it lets go.
type tableDrag struct {
	table  doc.ID
	kind   control
	from   int
	clicks int
	extend bool
	// x0, y0 are where it began and x, y where it is, in the page.
	x0, y0, x, y float32
	moved        bool
	// to is where a carried line lands, and offsets those of the lines
	// making room for it.
	to      int
	offsets []motion.Spring
	// rows and columns are the size a corner's drag sets; w0 the width a
	// border's began at.
	rows, columns int
	w0            float32
}

// tableMade is a row or column an area made: its outline fades into it.
type tableMade struct {
	table  doc.ID
	column bool
	index  int
	at     time.Time
}

// chromeScale is the controls' size over their default.
func (el *Element) chromeScale() float32 {
	if el.UIFontSize <= 0 {
		return 1
	}
	return el.UIFontSize / theme.DefaultUIFontSize
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
		top := u.y + t.rows[row] + t.dy[row]
		rr = ui.Rect{X: b.X - off - short, Y: top + (t.heights[row]-long)/2, W: short, H: long}
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

func contains(r ui.Rect, x, y float32) bool {
	return r.W > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func grown(r ui.Rect, dx, dy float32) ui.Rect { return outset(r, dx, dy) }

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

// chromeAt is the control of a table under a page point: a grip of the
// table the pointer is by, its open area, or a column's border.
func (el *Element) chromeAt(x, y float32) chromeHit {
	pl := el.Editor.layout
	if pl == nil {
		return chromeHit{}
	}
	ch := &el.chrome
	scale := el.chromeScale()
	for _, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		if u.pres.Unit == ch.near {
			rr, cr, corner := el.gripRects(u, ch.row, ch.column)
			switch {
			case contains(grown(corner, 3*scale, 3*scale), x, y):
				return chromeHit{u: u, kind: controlCorner}
			case contains(rr, x, y):
				return chromeHit{u: u, kind: controlRow, at: ch.row}
			case contains(cr, x, y):
				return chromeHit{u: u, kind: controlColumn, at: ch.column}
			}
		}
		if a, ok := t.areaOf(); ok && contains(a, x-u.x, y-u.y) {
			return chromeHit{u: u, kind: controlArea, at: t.in.at}
		}
		b := tableBox(u)
		if y < b.Y || y >= b.Y+b.H || x < b.X-borderGrab*scale || x >= b.X+b.W+borderGrab*scale {
			continue
		}
		for c := range t.columns {
			cell := t.cell(doc.CellCoord{Column: c})
			edge := u.x + cell.X + cell.W
			if math.Abs(float64(x-edge)) <= borderGrab*float64(scale) && edge >= b.X && edge <= b.X+b.W+1 {
				return chromeHit{u: u, kind: controlBorder, at: c}
			}
		}
	}
	return chromeHit{}
}

// followTables follows the pointer by the page's tables, as the last
// layout set them out: the table whose grips show and the lines it is
// over, and the areas it calls up, which it asks the next layout for.
func (el *Element) followTables(c *ui.Context, inside bool) {
	ch := &el.chrome
	ch.inside = inside
	pl := el.Editor.layout
	ed := el.Editor
	if pl == nil {
		return
	}
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	scale := el.chromeScale()
	free := inside && el.gesture == nil && ch.drag == nil && ed.resize == nil
	rowsAt, rowsOpen := ch.rows.Open()
	colsAt, colsOpen := ch.columns.Open()
	wantRows, wantCols := -1, -1
	rowNear, colNear := float32(math.Inf(1)), float32(math.Inf(1))
	ch.near, ch.row, ch.column = 0, -1, -1
	if d := ch.drag; d != nil {
		ch.near = d.table
	}
	for i, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		if d := ch.drag; d != nil && d.table == u.pres.Unit {
			ch.row, ch.column = el.linesAt(u, x, y)
			switch d.kind {
			case controlRow:
				ch.row = d.from
			case controlColumn:
				ch.column = d.from
			}
			continue
		}
		b := tableBox(u)
		open := rowsOpen && rowsAt/gapStride == i || colsOpen && colsAt/gapStride == i
		reach := tableReach * scale
		if open {
			reach = tableHold * scale
		}
		near := grown(b, reach, reach)
		if a, ok := t.areaOf(); ok {
			a.X += u.x
			a.Y += u.y
			near = union(near, grown(a, reach, reach))
			// An open area holds while the pointer is on it.
			over := contains(grown(a, tableBorder*scale, tableBorder*scale), x, y)
			if t.in.area == areaRow {
				ch.rows.HoldOver(over)
			} else {
				ch.columns.HoldOver(over)
			}
		}
		if !inside || !contains(near, x, y) || ch.drag != nil {
			continue
		}
		ch.near = u.pres.Unit
		ch.row, ch.column = el.linesAt(u, x, y)
		if !free {
			continue
		}
		rows, columns := len(t.rows), len(t.columns)
		// By the table's foot or its end, the area making a row or a
		// column there; where the two meet, the nearer edge's.
		edge, hold := tableEdge*scale, tableReach*scale
		if open {
			hold = tableHold * scale
		}
		bottom, right := b.Y+b.H, b.X+b.W
		foot := ui.Rect{X: b.X - edge, Y: bottom - edge, W: b.W + 2*edge, H: edge + hold}
		end := ui.Rect{X: right - edge, Y: b.Y - edge, W: edge + hold, H: b.H + 2*edge}
		inFoot, inEnd := contains(foot, x, y), contains(end, x, y)
		switch {
		case inEnd && (!inFoot || abs32(x-right) < abs32(y-bottom)):
			wantCols, colNear = i*gapStride+columns, 0
		case inFoot:
			wantRows, rowNear = i*gapStride+rows, 0
		}
		// Between two lines, from the gutter before the rows or above the
		// columns: never from over the cells, where the pointer is for the
		// text.
		gutter := tableReach * scale
		if x >= b.X-gutter && x < b.X {
			for r := 1; r < rows; r++ {
				if d := abs32(y - (u.y + t.rows[r])); d <= tableBorder*scale && d < rowNear {
					wantRows, rowNear = i*gapStride+r, d
				}
			}
		}
		if y >= b.Y-gutter && y < b.Y {
			for col := 1; col < columns; col++ {
				left := u.x + t.cell(doc.CellCoord{Column: col}).X
				if d := abs32(x - left); d <= tableBorder*scale && d < colNear {
					wantCols, colNear = i*gapStride+col, d
				}
			}
		}
	}
	if !free {
		ch.rows.Close()
		ch.columns.Close()
	}
	// One area opens at a time: the one open already, else the nearer.
	switch {
	case rowsOpen && wantRows >= 0:
		wantCols = -1
	case colsOpen && wantCols >= 0:
		wantRows = -1
	case wantRows >= 0 && wantCols >= 0 && rowNear <= colNear:
		wantCols = -1
	case wantRows >= 0 && wantCols >= 0:
		wantRows = -1
	}
	rowsAt, rowsOpen = ch.rows.Follow(c, wantRows)
	colsAt, colsOpen = ch.columns.Follow(c, wantCols)
	// What the layout is asked for: the open area's room, and the width a
	// border's drag sets.
	ins := map[doc.ID]tableIn{}
	for i, u := range pl.units {
		if u.table == nil {
			continue
		}
		var in tableIn
		switch {
		case rowsOpen && rowsAt/gapStride == i:
			in.area, in.at = areaRow, rowsAt%gapStride
		case colsOpen && colsAt/gapStride == i:
			in.area, in.at = areaColumn, colsAt%gapStride
		}
		if d := ch.drag; d != nil && d.table == u.pres.Unit && d.kind == controlBorder && d.moved {
			in.sized, in.width = d.from, max(d.w0+d.x-d.x0, leastColumn)
		}
		if in != (tableIn{}) {
			ins[u.pres.Unit] = in
		}
	}
	ed.tables = ins
}

func union(a, b ui.Rect) ui.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// byTable reports whether a page point is by a table, where its controls
// follow the pointer.
func (el *Element) byTable(x, y float32) bool {
	pl := el.Editor.layout
	if pl == nil {
		return false
	}
	reach := tableHold * el.chromeScale()
	for _, u := range pl.units {
		if u.table == nil {
			continue
		}
		r := grown(tableBox(u), reach, reach)
		if a, ok := u.table.areaOf(); ok {
			a.X += u.x
			a.Y += u.y
			r = union(r, grown(a, reach, reach))
		}
		if contains(r, x, y) || u.pres.Unit == el.chrome.near {
			return true
		}
	}
	return false
}

// chromeCursor is the cursor over a table's control at a page point.
func (el *Element) chromeCursor(x, y float32) (ui.Cursor, bool) {
	kind := controlNone
	if d := el.chrome.drag; d != nil {
		kind = d.kind
	} else {
		kind = el.chromeAt(x, y).kind
	}
	switch kind {
	case controlRow, controlColumn:
		if el.chrome.drag != nil {
			return ui.CursorGrabbing, true
		}
		return ui.CursorGrab, true
	case controlCorner:
		return ui.CursorResizeNWSE, true
	case controlBorder:
		return ui.CursorResizeEW, true
	case controlArea:
		return ui.CursorPointer, true
	}
	return 0, false
}

// beginTableDrag takes a press on a table's control.
func (el *Element) beginTableDrag(x, y float32, clicks int, extend bool) bool {
	h := el.chromeAt(x, y)
	if h.kind == controlNone {
		return false
	}
	d := &tableDrag{table: h.u.pres.Unit, kind: h.kind, from: h.at, clicks: clicks, extend: extend,
		x0: x, y0: y, x: x, y: y, to: h.at}
	t := h.u.table
	switch h.kind {
	case controlCorner:
		d.rows, d.columns = len(t.rows), len(t.columns)
	case controlBorder:
		d.w0 = t.widths[h.at]
	}
	el.chrome.drag = d
	return true
}

// moveTableDrag follows the pointer holding a control.
func (el *Element) moveTableDrag(x, y float32) bool {
	d := el.chrome.drag
	if d == nil {
		return false
	}
	d.x, d.y = x, y
	if !d.moved && max(abs32(x-d.x0), abs32(y-d.y0)) < dragSlop {
		return true
	}
	if d.kind == controlArea {
		// An area is clicked, not dragged.
		return true
	}
	d.moved = true
	if d.kind == controlCorner {
		if u := el.Editor.layout.unitOf(d.table); u != nil && u.table != nil {
			t := u.table
			rows, columns := len(t.rows), len(t.columns)
			cellW := t.columns[columns-1] + t.widths[columns-1]
			cellH := t.rows[rows-1] + t.heights[rows-1]
			d.columns = max(1, columns+int(math.Round(float64((x-d.x0)/(cellW/float32(columns))))))
			d.rows = max(1, rows+int(math.Round(float64((y-d.y0)/(cellH/float32(rows))))))
		}
	}
	return true
}

// finishTableDrag ends a press on a control: dropped when canceled, else
// its one intent.
func (el *Element) finishTableDrag(canceled bool, at time.Time) bool {
	d := el.chrome.drag
	if d == nil {
		return false
	}
	el.chrome.drag = nil
	el.Editor.tables = nil
	if canceled {
		return true
	}
	if in, ok := el.dragIntent(d, at); ok {
		el.Editor.Do(in, at)
	}
	return true
}

// dragIntent is what a press on a control does as it lets go.
func (el *Element) dragIntent(d *tableDrag, at time.Time) (vocab.Intent, bool) {
	u := el.Editor.layout.unitOf(d.table)
	if u == nil || u.table == nil {
		return nil, false
	}
	t := u.table
	shape := profile.TableShapeOf(u.pres.Node)
	switch d.kind {
	case controlRow, controlColumn:
		axis := profile.Rows
		if d.kind == controlColumn {
			axis = profile.Columns
		}
		if d.moved {
			if d.to == d.from {
				return nil, false
			}
			return profile.EditGrid{Op: profile.GridMove, Axis: axis, Table: d.table, At: d.from, Count: 1, To: d.to}, true
		}
		// A click picks the line's cells whole; with Shift, from the line
		// a pick of the same lines began at.
		cr := doc.CellRange{Table: d.table, WholeRows: axis == profile.Rows, WholeColumns: axis == profile.Columns}
		line := doc.CellCoord{Row: d.from, Column: d.from}
		cr.Anchor, cr.Head = line, line
		if sel := el.Editor.Selection(); d.extend && sel.Kind == doc.SelectCells && sel.Cells.Table == d.table &&
			sel.Cells.WholeRows == cr.WholeRows && sel.Cells.WholeColumns == cr.WholeColumns {
			cr.Anchor = sel.Cells.Anchor
		}
		return vocab.Place{Cells: &cr}, true
	case controlCorner:
		if !d.moved {
			return vocab.Place{Node: d.table}, true
		}
		if d.rows == len(t.rows) && d.columns == len(t.columns) {
			return nil, false
		}
		return profile.ResizeTable{Table: d.table, Rows: d.rows, Columns: d.columns}, true
	case controlBorder:
		var col profile.Column
		if d.from < len(shape.Columns) {
			col = shape.Columns[d.from]
		}
		switch {
		case d.clicks >= 2 && !d.moved:
			// A double click fits the column to its text, or lets it go.
			col.Fit, col.Width = !col.Fit, 0
		case d.moved:
			col.Width, col.Fit = int(math.Round(float64(max(d.w0+d.x-d.x0, leastColumn)))), false
		default:
			return nil, false
		}
		return profile.ShapeColumn{Table: d.table, Column: d.from, Shape: col}, true
	case controlArea:
		if d.moved {
			return nil, false
		}
		axis, n := profile.Rows, len(t.rows)
		gap := &el.chrome.rows
		if t.in.area == areaColumn {
			axis, n, gap = profile.Columns, len(t.columns), &el.chrome.columns
		}
		gap.Use()
		el.chrome.made = &tableMade{table: d.table, column: axis == profile.Columns, index: min(d.from, n), at: at}
		if d.from >= n {
			return profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: d.table, At: n - 1, Count: 1}, true
		}
		return profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: d.table, At: d.from, Count: 1, Before: true}, true
	}
	return nil, false
}

// carryTable draws the line a grip carries where the pointer holds it and
// the others making room for it, springing there, and the row drawn first.
// It reports whether any still moves.
func (el *Element) carryTable(u *unitLayout, now time.Time, frame, animate bool) bool {
	t := u.table
	clear(t.dx)
	clear(t.dy)
	t.lifted, t.lead = -1, 0
	d := el.chrome.drag
	if d == nil || d.table != u.pres.Unit || !d.moved || d.kind != controlRow && d.kind != controlColumn {
		return false
	}
	rows := d.kind == controlRow
	starts, sizes, offs, delta := t.rows, t.heights, t.dy, d.y-d.y0
	if !rows {
		starts, sizes, offs, delta = t.columns, t.widths, t.dx, d.x-d.x0
	}
	n, from := len(sizes), d.from
	if from < 0 || from >= n {
		return false
	}
	// Within the grid; it lands past each line its edge leading the way
	// has passed the middle of.
	delta = max(starts[0]-starts[from], min(delta, starts[n-1]+sizes[n-1]-starts[from]-sizes[from]))
	to := carryTo(starts, sizes, from, starts[from]+delta)
	d.to = to
	if len(d.offsets) != n {
		d.offsets = make([]motion.Spring, n)
		for k := range d.offsets {
			d.offsets[k] = motion.NewSpring(0, now)
		}
	}
	moving := false
	for k := range n {
		target := float32(0)
		switch {
		case k == from:
		case from < k && k <= to:
			target = -sizes[from]
		case to <= k && k < from:
			target = sizes[from]
		}
		s := &d.offsets[k]
		if frame && animate {
			s.Step(target, now, figureFade)
		} else if frame || !animate {
			*s = motion.NewSpring(target, now)
		}
		offs[k] = s.Value()
		moving = moving || s.Moving()
	}
	offs[from] = delta
	t.lifted, t.liftRows = from, rows
	if rows {
		switch {
		case to == 0:
			t.lead = from
		case from == 0:
			t.lead = 1
		}
	}
	return moving
}

// carryTo is where a line from carried to top lands: past each line before
// it whose middle its top has not passed going up, and each after it whose
// middle its bottom has passed going down.
func carryTo(starts, sizes []float32, from int, top float32) int {
	to := from
	for k := range starts {
		middle := starts[k] + sizes[k]/2
		switch {
		case k < from && top < middle:
			to--
		case k > from && top+sizes[from] > middle:
			to++
		}
	}
	return to
}

// paintTableChrome draws the controls of the tables the pointer is by,
// their open areas and what a drag would do, over the page. It reports
// whether any still moves.
func (el *Element) paintTableChrome(p *ui.Painter, pl *pageLayout, ox, oy float32, now time.Time, animate bool) bool {
	ch := &el.chrome
	if ch.grips == nil {
		ch.grips = map[doc.ID]*motion.Spring{}
	}
	if m := ch.made; m != nil && now.Sub(m.at) > madeFade {
		ch.made = nil
	}
	t := el.Theme
	scale := el.chromeScale()
	moving := false
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	seen := map[doc.ID]bool{}
	for _, u := range pl.units {
		tl := u.table
		if tl == nil {
			continue
		}
		id := u.pres.Unit
		seen[id] = true
		shown := float32(0)
		if ch.near == id && (ch.inside || ch.drag != nil) && el.Editor.resize == nil && el.gesture == nil {
			shown = 1
		}
		s, ok := ch.grips[id]
		if !ok {
			sp := motion.NewUnitSpring(0, now)
			s = &sp
			ch.grips[id] = s
		}
		if animate {
			s.Step(shown, now, figureFade)
		} else {
			*s = motion.NewUnitSpring(shown, now)
		}
		moving = moving || s.Moving()
		hover := el.chromeAt(x, y)
		if a := s.Value(); a > 0.001 {
			rr, cr, corner := el.gripRects(u, ch.row, ch.column)
			paintGrip := func(r ui.Rect, kind control, icon components.IconName) {
				if r.W <= 0 {
					return
				}
				r.X += ox
				r.Y += oy
				held := ch.drag != nil && ch.drag.table == id && ch.drag.kind == kind
				over := hover.u == u && hover.kind == kind || held
				if icon == "" {
					fill := t.Surface.Elevated
					if over {
						fill = t.Accent.Foreground
					}
					p.Shadow(r, r.W/2, 0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.2*a))
					p.Fill(r, fill.Alpha(a), r.W/2)
					p.Stroke(r, t.Border.Default.Alpha(a), r.W/2, 1)
					return
				}
				ink := t.Icon.Muted
				if over {
					p.Fill(r, t.GhostElement.Hover.Alpha(a), 4*scale)
					ink = t.Icon.Default
				}
				if held {
					ink = t.Accent.Foreground
				}
				size := gripIcon * scale
				p.Icon(icon.SVG(), ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, ink.Alpha(a))
			}
			paintGrip(rr, controlRow, components.IconGrip)
			paintGrip(cr, controlColumn, components.IconGripAcross)
			paintGrip(corner, controlCorner, "")
		}
		if a, ok := tl.areaOf(); ok {
			a.X += u.x + ox
			a.Y += u.y + oy
			over := hover.u == u && hover.kind == controlArea
			line, ink := t.Border.Default, t.Text.Muted
			if over {
				p.Fill(a, t.Accent.Muted.Alpha(0.6), objectRadius(el.Editor.metrics))
				line, ink = t.Accent.Foreground, t.Accent.Foreground
			}
			components.DashedOutline(p, a, objectRadius(el.Editor.metrics), 1, line)
			size := min(16*scale, a.W*0.8, a.H*0.8)
			p.Icon(components.IconPlus.SVG(), ui.Rect{X: a.X + (a.W-size)/2, Y: a.Y + (a.H-size)/2, W: size, H: size}, ink)
		}
		if m := ch.made; m != nil && m.table == id {
			// What an area made shows the area it was, fading into it.
			var r ui.Rect
			if m.column && m.index < len(tl.columns) {
				r = tl.cell(doc.CellCoord{Column: m.index})
				r.Y, r.H = u.box.Y, u.box.H
			} else if !m.column && m.index < len(tl.rows) {
				r = tl.cell(doc.CellCoord{Row: m.index})
				r.X, r.W = u.box.X, u.box.W
			}
			if r.W > 0 {
				r.X += u.x + ox
				r.Y += u.y + oy
				f := 1 - float32(now.Sub(m.at))/float32(madeFade)
				f = f * f * (3 - 2*f)
				p.Fill(r, t.Accent.Muted.Alpha(0.6*f), objectRadius(el.Editor.metrics))
				components.DashedOutline(p, r, objectRadius(el.Editor.metrics), 1, t.Accent.Foreground.Alpha(f))
				moving = true
			}
		}
		if d := ch.drag; d != nil && d.table == id && d.moved {
			b := tableBox(u)
			b.X += ox
			b.Y += oy
			switch d.kind {
			case controlCorner:
				// The size it would take, outlined and named.
				rows, columns := len(tl.rows), len(tl.columns)
				w := b.W * float32(d.columns) / float32(columns)
				h := b.H * float32(d.rows) / float32(rows)
				r := ui.Rect{X: b.X, Y: b.Y, W: w, H: h}
				components.DashedOutline(p, r, objectRadius(el.Editor.metrics), 1.5, t.Accent.Foreground)
				// Named under its far corner, clear of the cells.
				el.paintChip(p, fmt.Sprintf("%d × %d", d.rows, d.columns), func(chip ui.Rect) (ui.Rect, bool) {
					// The view's right edge, in the paint's frame.
					right := ox - el.column + el.viewport.W - 8*scale
					chip.X, chip.Y = min(r.X+r.W, right)-chip.W, r.Y+r.H+4*scale
					return chip, true
				})
			case controlBorder:
				c := tl.cell(doc.CellCoord{Column: d.from})
				edge := u.x + ox + c.X + c.W
				p.Fill(ui.Rect{X: edge - 1, Y: b.Y, W: 2, H: b.H}, t.Accent.Foreground, 1)
			}
		}
	}
	for id := range ch.grips {
		if !seen[id] {
			delete(ch.grips, id)
		}
	}
	return moving
}

// placeForMenu puts the caret in the cell a secondary click is on, unless
// the selection holds it already, so the table's commands act there.
func (el *Element) placeForMenu(x, y float32, at time.Time) {
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	c, _, ok := pl.hit(x, y)
	if !ok {
		return
	}
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

// tableMenu are the table commands a cell's menu offers, in its groups.
var tableMenu = [][]vocab.CommandID{
	{"table.row-above", "table.row-below", "table.column-left", "table.column-right"},
	{"table.align-start", "table.align-center", "table.align-end", "table.header"},
	{"table.delete-row", "table.delete-column", "table.delete"},
}

// contextMenu is the page's menu: a table's commands where the selection
// is in one, each through the command table and offered as it would act,
// then the text's editing items.
func (el *Element) contextMenu(m *ui.Menu, services ui.Services) {
	ed := el.Editor
	labels := map[vocab.CommandID]string{}
	for _, c := range keymap.Declared(ed.Engine.Program().Catalog()) {
		labels[vocab.CommandID(c.ID)] = c.Label
	}
	shown := false
	for _, group := range tableMenu {
		var items []vocab.CommandID
		for _, id := range group {
			out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: vocab.Command{ID: id}, At: ed.active})
			if err == nil && out.Available {
				items = append(items, id)
			}
		}
		if len(items) == 0 {
			continue
		}
		if shown {
			m.Separator()
		}
		shown = true
		for _, id := range items {
			if m.Item(labels[id]).Chosen() {
				el.run(vocab.Command{ID: id}, services, time.Now())
			}
		}
	}
	if shown {
		m.Separator()
	}
	m.EditItems()
}

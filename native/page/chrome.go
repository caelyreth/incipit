package page

import (
	"fmt"
	"maps"
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
//     picks its cells whole (a Place carrying the cell range). Dragged, it
//     lifts a copy of the line under the pointer, the line dimmed in its
//     place; where it would land, once the pointer has stayed a moment,
//     the grid parts, a frame its size opening there as the one it leaves
//     closes. Where it would land is read from where the lines stood as
//     the drag began, so nothing chases the pointer. Letting go moves it
//     there (EditGrid), the copy landing in its place as the room it left
//     closes behind it. The knob at the table's far corner drags its size,
//     outlined and named as it goes and kept on letting go (ResizeTable);
//     a click on it selects the table. Escape drops any drag.
//   - A column's border lights under the pointer and drags its width,
//     previewed as the layout sets it; a double click fits the column to
//     its text (ShapeColumn).
//   - The pointer resting by the table's foot opens a dashed area under it
//     that makes a row, and by its end one beside it that makes a column;
//     in the gutter by a border between two rows, or above the table by
//     one between two columns, an area opens between them, the lines
//     gliding apart and the page lifting by half the room, so the border
//     stays about the pointer.
//   - Every line a table gains comes in, from nothing or from the size of
//     the area that made it, whose outline fades into it.
//   - A secondary click on a cell opens the table's commands, each through
//     the command table.
//
// What the controls open is room the layout gives as it is asked
// (tableIn), each room as far as it has come; the page steps them at its
// paint's time, as it does every spring.
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
	// it off what it moves; cornerSize is the corner's knob, borderGrab how
	// near a column's border a press takes it, and borderRing how wide it
	// lights.
	gripLong, gripShort, gripIcon, gripOff = 22, 16, 14, 4
	cornerSize                             = 10
	borderGrab                             = 4
	borderRing                             = 2
	// dragSlop is how far a press goes before it is a drag.
	dragSlop = 3
	// gapStride numbers an area by its table and its place.
	gapStride = 1 << 12
	// carryVeil is how far a carried line is hidden in its place.
	carryVeil = 0.6
)

const (
	// madeFade is how long a made line shows the area it was, carryLand
	// how long a carried line's copy takes to land, and carrySettle how
	// long the pointer stays by a place before the grid parts there.
	madeFade    = 2 * components.MotionNormal
	carryLand   = components.MotionNormal
	carrySettle = 120 * time.Millisecond
)

// The chrome's springs arrive as the app's do (components.Spring): rooms,
// grips and lights in MotionNormal, a carry's parting in MotionPanel, a
// hover in HoverIn and out in HoverOut, a press in MotionImmediate.
var (
	chromeFollow = settleFrequency(components.MotionNormal)
	carryPart    = settleFrequency(components.MotionPanel)
	hoverIn      = settleFrequency(components.HoverIn)
	hoverOut     = settleFrequency(components.HoverOut)
	pressFollow  = settleFrequency(components.MotionImmediate)
)

// settleFrequency is the frequency of a critically damped spring whose
// undamped period is d, as the app's springs take their duration.
func settleFrequency(d time.Duration) float32 { return float32(2 * math.Pi / d.Seconds()) }

// follow steps a spring toward target at now, there at once without
// motion, and reports whether it still moves.
func follow(s *motion.Spring, target float32, now time.Time, frequency float32, animate, unit bool) bool {
	if animate {
		s.Step(target, now, frequency)
		return s.Moving()
	}
	if unit {
		*s = motion.NewUnitSpring(target, now)
	} else {
		*s = motion.NewSpring(target, now)
	}
	return false
}

// control is what of a table's controls a place is on.
type control uint8

const (
	controlNone control = iota
	controlRow
	controlColumn
	controlCorner
	controlBorder
	// The areas: under the foot, beside the end, between two rows and
	// between two columns.
	controlBelow
	controlBeside
	controlRowArea
	controlColumnArea
	controlCount
)

func (k control) area() bool { return k >= controlBelow && k < controlCount }

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
	// below are the areas at a table's foot, numbered twice its place, and
	// at its end, once more; rows and columns those between two lines,
	// numbered by gapStride.
	below, rows, columns components.CreateGap
	drag                 *tableDrag
	landing              *tableLanding
	made                 *tableMade
	motions              map[doc.ID]*tableMotion
	// lift is how far the page is scrolled to open room between two rows
	// about the pointer.
	lift float32
	// menu is the commands the page's menu offers, as its secondary click
	// chose them by where it was.
	menu [][]vocab.CommandID
}

// tableMotion is a table's springs: the rooms its areas open, each where
// it stands; how far its grips, the area beside it and a border's light
// have come in; the grips' places and every control's hover; the lines
// coming in, and the rooms a carry opens.
type tableMotion struct {
	below, beside, rowRoom, columnRoom motion.Spring
	row, column                        int
	grips, border, press               motion.Spring
	borderAt                           int
	hover                              [controlCount]motion.Spring
	// grip are the row's, the column's and the corner's grips as drawn.
	grip  [3]gripPlace
	cells map[doc.ID]bool
	grow  [2]growth
	apart []*apartRoom
}

// gripPlace is a grip where it is drawn, gliding to where it goes.
type gripPlace struct {
	x, y  motion.Spring
	shown bool
}

// growth is a run of lines coming in.
type growth struct {
	from, to int
	spring   motion.Spring
}

// apartRoom is room a carry opens before line at, as far as target, and
// whether a frame shows in it where the carried line would land.
type apartRoom struct {
	rows   bool
	at     int
	target float32
	frame  bool
	spring motion.Spring
}

func newTableMotion(now time.Time) *tableMotion {
	tm := &tableMotion{row: -1, column: -1, beside: motion.NewUnitSpring(0, now), grips: motion.NewUnitSpring(0, now),
		border: motion.NewUnitSpring(0, now), press: motion.NewUnitSpring(0, now),
		below: motion.NewSpring(0, now), rowRoom: motion.NewSpring(0, now), columnRoom: motion.NewSpring(0, now)}
	for k := range tm.hover {
		tm.hover[k] = motion.NewUnitSpring(0, now)
	}
	return tm
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
	// A carry: start is the line's box as it began, in the unit, mids the
	// middles of the lines then and size its size, along the way it
	// moves; over is the place the pointer is by since since, and to the
	// place the grid parts at, each before the line of its number.
	start ui.Rect
	mids  []float32
	size  float32
	over  int
	since time.Time
	to    int
	// rows and columns are the size a corner's drag sets; w0 the width a
	// border's began at.
	rows, columns int
	w0            float32
}

// moves reports whether landing before line slot moves the carried line:
// either side of its own place is where it is.
func (d *tableDrag) moves(slot int) bool { return slot != d.from && slot != d.from+1 }

// carrying reports whether the drag carries a line, a row if rows.
func (d *tableDrag) carrying() (rows, ok bool) {
	return d.kind == controlRow, d.moved && (d.kind == controlRow || d.kind == controlColumn)
}

// at is where the carried copy's corner is, in the unit.
func (d *tableDrag) at() ui.Point {
	if d.kind == controlRow {
		return ui.Point{X: d.start.X, Y: d.start.Y + d.y - d.y0}
	}
	return ui.Point{X: d.start.X + d.x - d.x0, Y: d.start.Y}
}

// carryIndex is where a line from goes, landing before slot.
func carryIndex(from, slot int) int {
	if slot > from {
		return slot - 1
	}
	return slot
}

// tableLanding is a carried line's copy on its way from where it was let
// go, its corner in the unit, into its place.
type tableLanding struct {
	table doc.ID
	rows  bool
	index int
	from  ui.Point
	at    time.Time
}

// tableMade is a line an area made: it comes in from the area's size, as
// a share of its own, the area's outline fading into it.
type tableMade struct {
	table  doc.ID
	column bool
	index  int
	from   float32
	at     time.Time
}

// chromeSize is the UI size the chrome is set at, and chromeScale its
// share of the default.
func (el *Element) chromeSize() float32 {
	if el.UIFontSize > 0 {
		return el.UIFontSize
	}
	return theme.DefaultUIFontSize
}

func (el *Element) chromeScale() float32 { return el.chromeSize() / theme.DefaultUIFontSize }

func (el *Element) controls() theme.ControlMetrics {
	return theme.DefaultControls.Scaled(el.chromeSize())
}

// tableGutter is the room a table's controls keep in each margin, which a
// table wider than the column leaves them.
func (el *Element) tableGutter() float32 {
	return tableReach*el.chromeScale() + el.controls().Height
}

// areaRooms are the gap an area keeps from the table, and from the lines
// either side of it between two; the room under the foot, an area and its
// gap; and the room between two lines, an area and its gap each side.
func (el *Element) areaRooms() (gap, inner, step, between float32) {
	size, h := el.chromeSize(), el.controls().Height
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
	kind               control
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
		out = append(out, tableArea{kind: controlBelow, alpha: reveal, icon: reveal, label: components.Smoothstep(reveal),
			box: ui.Rect{X: u.x + b.X, Y: u.y + b.Y + gap, W: b.W, H: max(1, b.H-gap)}})
	}
	if tm := el.chrome.motions[u.pres.Unit]; tm != nil {
		if reveal := tm.beside.Value(); reveal > 0.001 {
			b := tableBox(u)
			out = append(out, tableArea{kind: controlBeside, alpha: reveal, icon: 1, label: 1, shift: (1 - reveal) * -gap, compact: true,
				box: ui.Rect{X: b.X + b.W + gap, Y: b.Y, W: el.controls().Height, H: b.H}})
		}
	}
	if g := t.rowGap; g.H > 1 {
		reveal := min(1, g.H/between)
		out = append(out, tableArea{kind: controlRowArea, alpha: reveal, icon: reveal, label: components.Smoothstep(reveal),
			box: ui.Rect{X: u.x + g.X, Y: u.y + g.Y + inner, W: g.W, H: max(1, g.H-2*inner)}})
	}
	if g := t.across(t.columnGap); g.W > 1 {
		reveal := min(1, g.W/between)
		out = append(out, tableArea{kind: controlColumnArea, alpha: reveal, icon: reveal, label: 1, compact: true,
			box: ui.Rect{X: u.x + g.X + inner, Y: u.y + g.Y, W: max(1, g.W-2*inner), H: g.H}})
	}
	return out
}

func contains(r ui.Rect, x, y float32) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
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
// table the pointer is by, an area it shows, or a column's border.
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
		for _, a := range el.tableAreas(u) {
			if a.alpha > 0.5 && contains(a.box, x, y) {
				return chromeHit{u: u, kind: a.kind}
			}
		}
		b := tableBox(u)
		if y < b.Y || y >= b.Y+b.H || x < b.X-borderGrab*scale || x >= b.X+b.W+borderGrab*scale {
			continue
		}
		for c := range t.columns {
			cell := t.cell(doc.CellCoord{Column: c})
			edge := u.x + cell.X + cell.W
			if abs32(x-edge) <= borderGrab*scale && edge >= b.X && edge <= b.X+b.W+1 {
				return chromeHit{u: u, kind: controlBorder, at: c}
			}
		}
	}
	return chromeHit{}
}

// followTables follows the pointer by the page's tables, as the last
// layout set them out: the table whose grips show and the lines it is
// over, and the areas it calls up, each opening once the pointer has
// rested by it.
func (el *Element) followTables(c *ui.Context, inside bool) {
	ch := &el.chrome
	ch.inside = inside
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	scale := el.chromeScale()
	gap := theme.Space06.DIPs(el.chromeSize())
	beside := gap + el.controls().Height
	free := inside && el.gesture == nil && ch.drag == nil && el.Editor.resize == nil
	nearAt, nearOpen := ch.below.Open()
	edgeAt, rowAt, columnAt := -1, -1, -1
	rowNear, columnNear := float32(0), float32(0)
	ch.near, ch.row, ch.column = 0, -1, -1
	d := ch.drag
	if d != nil {
		ch.near = d.table
	}
	if _, ok := d.carryingOK(); ok {
		// A carry chooses where to land: nothing is made meanwhile.
		ch.below.Close()
		ch.rows.Close()
		ch.columns.Close()
	}
	for i, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		if d != nil {
			if d.table == u.pres.Unit {
				ch.row, ch.column = el.linesAt(u, x, y)
				switch d.kind {
				case controlRow:
					ch.row = d.from
				case controlColumn:
					ch.column = d.from
				}
			}
			continue
		}
		b := tableBox(u)
		reach := tableReach * scale
		if nearOpen && nearAt/2 == i {
			reach = tableHold * scale
		}
		// The table, its grips before it and above it, the area beside
		// it and the one under it.
		near := grown(b, reach, reach)
		near.W += beside
		near.H += t.below.H
		if !inside || !contains(near, x, y) {
			continue
		}
		ch.near = u.pres.Unit
		ch.row, ch.column = el.linesAt(u, x, y)
		if !free {
			continue
		}
		// The areas making a row and a column are called up by the
		// table's foot and its end, not from over its cells: the pointer
		// just within the edge, or past it. Where the two meet, the one
		// open holds, else the nearer edge's.
		hold := func(side int) float32 {
			if nearOpen && nearAt == 2*i+side {
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
		case inFoot && inEnd && nearOpen && nearAt/2 == i:
			edgeAt = nearAt
		case inEnd && (!inFoot || abs32(x-right) < abs32(y-bottom)):
			edgeAt = 2*i + 1
		case inFoot:
			edgeAt = 2 * i
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
				rowAt, rowNear = i*gapStride+r, abs32(y-at)
				break
			}
		}
		for col := 1; col < len(t.columns); col++ {
			at := u.x + t.cell(doc.CellCoord{Column: col}).X
			if inColumns && abs32(x-at) <= border || contains(columnGap, x, y) && t.columnGap.X < t.columns[col] {
				columnAt, columnNear = i*gapStride+col, abs32(x-at)
				break
			}
		}
	}
	// One room opens between lines at a time: by a crossing of a row's
	// border and a column's, the one open already, else the nearer.
	_, rowsOpen := ch.rows.Open()
	_, columnsOpen := ch.columns.Open()
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
	ch.below.Follow(c, edgeAt)
	ch.rows.Follow(c, rowAt)
	ch.columns.Follow(c, columnAt)
	// An open area holds while the pointer is on it.
	if !inside {
		return
	}
	for _, u := range pl.units {
		if u.table == nil {
			continue
		}
		for _, a := range el.tableAreas(u) {
			if !contains(grown(a.box, tableBorder*scale, tableBorder*scale), x, y) {
				continue
			}
			switch a.kind {
			case controlBelow, controlBeside:
				ch.below.HoldOver(true)
			case controlRowArea:
				ch.rows.HoldOver(true)
			case controlColumnArea:
				ch.columns.HoldOver(true)
			}
		}
	}
}

// carryingOK is carrying on a drag that may be nil.
func (d *tableDrag) carryingOK() (bool, bool) {
	if d == nil {
		return false, false
	}
	return d.carrying()
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
		r.W += el.controls().Height
		r.H += u.table.below.H
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
	switch {
	case kind == controlRow, kind == controlColumn:
		if el.chrome.drag != nil {
			return ui.CursorGrabbing, true
		}
		return ui.CursorGrab, true
	case kind == controlCorner:
		return ui.CursorResizeNWSE, true
	case kind == controlBorder:
		return ui.CursorResizeEW, true
	case kind.area():
		return ui.CursorPointer, true
	}
	return 0, false
}

// beginTableDrag takes a press on a table's control.
func (el *Element) beginTableDrag(x, y float32, clicks int, extend bool, at time.Time) bool {
	h := el.chromeAt(x, y)
	if h.kind == controlNone {
		return false
	}
	d := &tableDrag{table: h.u.pres.Unit, kind: h.kind, from: h.at, clicks: clicks, extend: extend,
		x0: x, y0: y, x: x, y: y, over: h.at, to: h.at, since: at}
	t := h.u.table
	switch h.kind {
	case controlRow, controlColumn:
		rows := h.kind == controlRow
		d.start = t.line(h.u, rows, h.at)
		n := len(t.columns)
		if rows {
			n = len(t.rows)
		}
		for i := range n {
			r := t.line(h.u, rows, i)
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
	case controlCorner:
		d.rows, d.columns = len(t.rows), len(t.columns)
	case controlBorder:
		d.w0 = t.widths[h.at]
	}
	el.chrome.drag = d
	return true
}

// moveTableDrag follows the pointer holding a control.
func (el *Element) moveTableDrag(x, y float32, at time.Time) bool {
	d := el.chrome.drag
	if d == nil {
		return false
	}
	d.x, d.y = x, y
	if !d.moved && max(abs32(x-d.x0), abs32(y-d.y0)) < dragSlop {
		return true
	}
	if d.kind.area() {
		// An area is clicked, not dragged.
		return true
	}
	d.moved = true
	switch d.kind {
	case controlRow, controlColumn:
		// The place it is by, by its middle among the lines' middles as
		// they stood as it began.
		corner := d.at()
		middle := corner.Y + d.size/2
		if d.kind == controlColumn {
			middle = corner.X + d.size/2
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
	case controlCorner:
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
// its one intent. A carried line's copy lands either way.
func (el *Element) finishTableDrag(canceled bool, at time.Time) bool {
	d := el.chrome.drag
	if d == nil {
		return false
	}
	el.chrome.drag = nil
	if rows, ok := d.carrying(); ok {
		el.dropCarry(d, rows, canceled, at)
		return true
	}
	if canceled {
		return true
	}
	if in, ok := el.dragIntent(d, at); ok {
		el.Editor.Do(in, at)
	}
	return true
}

// dropCarry lets a carried line go: where the pointer is, in one step of
// history, unless canceled or it would land where it is, even where the
// pointer let go before the grid parted for it. Its copy lands from where
// it was let go; the frame it would land in goes at once, as the line
// stands there, and the room it left closes behind it.
func (el *Element) dropCarry(d *tableDrag, rows, canceled bool, at time.Time) {
	ch := &el.chrome
	d.to = d.over
	landing := &tableLanding{table: d.table, rows: rows, index: d.from, from: d.at(), at: at}
	ch.landing = landing
	tm := ch.motions[d.table]
	if tm != nil {
		for _, a := range tm.apart {
			if a.frame && a.target > 0 {
				a.target, a.spring = 0, motion.NewSpring(0, at)
			}
			a.frame = false
		}
	}
	if canceled || !d.moves(d.to) {
		return
	}
	axis := profile.Columns
	if rows {
		axis = profile.Rows
	}
	to := carryIndex(d.from, d.to)
	el.Editor.Do(profile.EditGrid{Op: profile.GridMove, Axis: axis, Table: d.table, At: d.from, Count: 1, To: to}, at)
	landing.index = to
	if tm == nil {
		return
	}
	left := d.from
	if d.to < d.from {
		left = d.from + 1
	}
	a := tm.apartRoom(rows, left)
	a.target, a.frame, a.spring = 0, false, motion.NewSpring(d.size, at)
}

// apartRoom is the room a carry opens before line at.
func (tm *tableMotion) apartRoom(rows bool, at int) *apartRoom {
	for _, a := range tm.apart {
		if a.rows == rows && a.at == at {
			return a
		}
	}
	a := &apartRoom{rows: rows, at: at}
	tm.apart = append(tm.apart, a)
	return a
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
	}
	if !d.kind.area() || d.moved {
		return nil, false
	}
	return el.useArea(u, d.kind, at)
}

// useArea makes the line an area makes, where it stands: the area's room
// goes at once, the line coming in from its size, and the page keeps where
// the room lifted it.
func (el *Element) useArea(u *unitLayout, kind control, at time.Time) (vocab.Intent, bool) {
	ch := &el.chrome
	t, id := u.table, u.pres.Unit
	tm := ch.motions[id]
	if tm == nil {
		return nil, false
	}
	rows, columns := len(t.rows), len(t.columns)
	rowShare := func(size float32) float32 { return min(1, size/max(1, t.heights[rows-1])) }
	axis, index, share := profile.Rows, rows, float32(0)
	switch kind {
	case controlBelow:
		ch.below.Use()
		index, share = rows, rowShare(t.below.H)
		tm.below = motion.NewSpring(0, at)
	case controlBeside:
		ch.below.Use()
		axis, index = profile.Columns, columns
		tm.beside = motion.NewUnitSpring(0, at)
	case controlRowArea:
		ch.rows.Use()
		index, share = tm.row, rowShare(t.rowGap.H)
		ch.lift -= tm.rowRoom.Value() / 2
		tm.rowRoom = motion.NewSpring(0, at)
	case controlColumnArea:
		ch.columns.Use()
		axis, index = profile.Columns, tm.column
		share = min(1, t.columnGap.W*float32(columns+1)/max(1, u.box.W))
		tm.columnRoom = motion.NewSpring(0, at)
	}
	n := rows
	if axis == profile.Columns {
		n = columns
	}
	ch.made = &tableMade{table: id, column: axis == profile.Columns, index: index, from: share, at: at}
	if index >= n {
		return profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: id, At: n - 1, Count: 1}, true
	}
	return profile.EditGrid{Op: profile.GridInsert, Axis: axis, Table: id, At: index, Count: 1, Before: true}, true
}

// stepTables moves each table's controls on to now: the rooms its areas
// and a carry open, its lines coming in, its grips and lights. What the
// layout is asked for changes as they do, and the page lays out again;
// the page lifts by half the room open between two rows. It reports
// whether any still moves.
func (el *Element) stepTables(now time.Time, animate bool) bool {
	ed := el.Editor
	pl := ed.layout
	ch := &el.chrome
	if pl == nil {
		return false
	}
	if ch.motions == nil {
		ch.motions = map[doc.ID]*tableMotion{}
	}
	if m := ch.made; m != nil && now.Sub(m.at) > madeFade {
		ch.made = nil
	}
	_, _, step, between := el.areaRooms()
	belowAt, belowOpen := ch.below.Open()
	rowsAt, rowsOpen := ch.rows.Open()
	columnsAt, columnsOpen := ch.columns.Open()
	hover := el.chromeAt(el.toPage(el.pointer.X, el.pointer.Y))
	open := func(is bool, v float32) float32 {
		if is {
			return v
		}
		return 0
	}
	moving := false
	lift := float32(0)
	ins := map[doc.ID]tableIn{}
	seen := map[doc.ID]bool{}
	for i, u := range pl.units {
		t := u.table
		if t == nil {
			continue
		}
		id := u.pres.Unit
		seen[id] = true
		tm := ch.motions[id]
		if tm == nil {
			tm = newTableMotion(now)
			ch.motions[id] = tm
		}
		d := ch.drag
		if d != nil && d.table != id {
			d = nil
		}
		moving = follow(&tm.below, open(belowOpen && belowAt == 2*i, step), now, chromeFollow, animate, false) || moving
		moving = follow(&tm.beside, open(belowOpen && belowAt == 2*i+1, 1), now, chromeFollow, animate, true) || moving
		if rowsOpen && rowsAt/gapStride == i {
			tm.row = rowsAt % gapStride
		}
		moving = follow(&tm.rowRoom, open(rowsOpen && rowsAt/gapStride == i, between), now, chromeFollow, animate, false) || moving
		if columnsOpen && columnsAt/gapStride == i {
			tm.column = columnsAt % gapStride
		}
		moving = follow(&tm.columnRoom, open(columnsOpen && columnsAt/gapStride == i, between), now, chromeFollow, animate, false) || moving
		shown := ch.near == id && (ch.inside || d != nil) && ed.resize == nil && el.gesture == nil
		moving = follow(&tm.grips, open(shown, 1), now, chromeFollow, animate, true) || moving
		// A border lights under the pointer and as its drag holds it.
		lit := false
		switch {
		case d != nil && d.kind == controlBorder:
			lit, tm.borderAt = true, d.from
		case ch.drag == nil && hover.u == u && hover.kind == controlBorder:
			lit, tm.borderAt = true, hover.at
		}
		moving = follow(&tm.border, open(lit, 1), now, chromeFollow, animate, true) || moving
		// Each control's hover eases in quickly and out gently; a held
		// one stays lit.
		for k := range tm.hover {
			on := hover.u == u && hover.kind == control(k) && (ch.drag == nil || d != nil && d.kind == control(k)) ||
				d != nil && d.kind == control(k)
			frequency := hoverOut
			if on {
				frequency = hoverIn
			}
			moving = follow(&tm.hover[k], open(on, 1), now, frequency, animate, true) || moving
		}
		moving = follow(&tm.press, open(d != nil && d.kind.area(), 1), now, pressFollow, animate, true) || moving
		moving = el.growLines(u, tm, now, animate) || moving
		moving = el.stepApart(tm, d, now, animate) || moving
		if tm.row > 0 && tm.row < len(t.rows) {
			lift += tm.rowRoom.Value() / 2
		}
		ins[id] = tm.asked(t, d)
		if ins[id] == (tableIn{}) {
			delete(ins, id)
		}
	}
	for id := range ch.motions {
		if !seen[id] {
			delete(ch.motions, id)
		}
	}
	// What is about the pointer stays under it as room opens between two
	// rows, a glide under way carried along.
	if delta := lift - ch.lift; delta != 0 {
		ch.lift = lift
		el.scroll += delta
		el.glide.Shift(delta)
	}
	if !maps.Equal(ins, ed.tables) {
		ed.tables = ins
		el.relayout()
	}
	return moving
}

// asked is what a table's layout is asked for as its springs stand, and a
// border's drag.
func (tm *tableMotion) asked(t *tableLayout, d *tableDrag) tableIn {
	var in tableIn
	if v := tm.below.Value(); v >= 0.5 {
		in.below = v
	}
	if v := tm.rowRoom.Value(); v >= 0.5 {
		in.row, in.rowRoom = tm.row, v
	}
	if v := tm.columnRoom.Value(); v >= 0.5 {
		in.column, in.columnRoom = tm.column, v
	}
	k := 0
	for _, a := range tm.apart {
		if v := a.spring.Value(); v >= 0.5 && k < maxApart {
			in.apart[k] = lineRoom{rows: a.rows, at: a.at, room: v}
			k++
		}
	}
	for axis, g := range tm.grow {
		if g.to > g.from {
			in.grow[axis] = lineGrow{from: g.from, to: g.to, share: g.spring.Value()}
		}
	}
	if d != nil && d.kind == controlBorder && d.moved {
		in.sized, in.width = d.from, max(d.w0+d.x-d.x0, leastColumn)
	}
	return in
}

// growLines brings in the lines a table has gained since it was last laid
// out: a run of rows, or of columns, whose cells are all new, from the
// share of its size the area that made it had, else from nothing.
func (el *Element) growLines(u *unitLayout, tm *tableMotion, now time.Time, animate bool) bool {
	t := u.table
	rows, columns := len(t.rows), len(t.columns)
	cells := make(map[doc.ID]bool, len(t.cells))
	freshRow, freshColumn := make([]bool, rows), make([]bool, columns)
	for r := range freshRow {
		freshRow[r] = true
	}
	for c := range freshColumn {
		freshColumn[c] = true
	}
	for i, cc := range t.cells {
		if cc.Row < 0 || i >= len(u.pres.Surfaces) {
			continue
		}
		cell := u.pres.Surfaces[i].Flow.Slot.Node
		cells[cell] = true
		if tm.cells[cell] {
			freshRow[cc.Row], freshColumn[cc.Column] = false, false
		}
	}
	if tm.cells != nil && animate {
		for axis, fresh := range [2][]bool{freshRow, freshColumn} {
			from, to := -1, -1
			for k, f := range fresh {
				if f {
					if from < 0 {
						from = k
					}
					to = k + 1
				}
			}
			// A table wholly new comes in as a unit does.
			if from < 0 || from == 0 && to == len(fresh) {
				continue
			}
			start := float32(0)
			if m := el.chrome.made; m != nil && m.table == u.pres.Unit && m.column == (axis == 1) && m.index == from {
				start = m.from
			}
			tm.grow[axis] = growth{from: from, to: to, spring: motion.NewUnitSpring(start, now)}
		}
	}
	tm.cells = cells
	moving := false
	for axis := range tm.grow {
		g := &tm.grow[axis]
		if g.to <= g.from {
			continue
		}
		if follow(&g.spring, 1, now, figureFade, animate, true) {
			moving = true
		} else {
			*g = growth{}
		}
	}
	return moving
}

// stepApart parts the grid where a carried line would land, once the
// pointer has stayed by the place a moment, the frame it leaves folding
// away, and lets each room close that is no longer asked for.
func (el *Element) stepApart(tm *tableMotion, d *tableDrag, now time.Time, animate bool) bool {
	moving := false
	if rows, ok := d.carryingOK(); ok {
		if d.over != d.to {
			if left := carrySettle - now.Sub(d.since); animate && left > 0 {
				moving = true
			} else {
				d.to = d.over
			}
		}
		for _, a := range tm.apart {
			if a.frame && (a.rows != rows || a.at != d.to) {
				a.target = 0
			}
		}
		if d.moves(d.to) {
			a := tm.apartRoom(rows, d.to)
			a.target, a.frame = d.size, true
		}
	}
	kept := tm.apart[:0]
	for _, a := range tm.apart {
		if follow(&a.spring, a.target, now, carryPart, animate, false) {
			moving = true
		}
		if a.target > 0 || a.spring.Value() >= 0.5 {
			kept = append(kept, a)
		}
	}
	tm.apart = kept
	return moving
}

// paintTableChrome draws the controls of the page's tables over the page:
// their areas, a made line's outline, a carry, the grips of the one the
// pointer is by, a lit border and the size a corner's drag would set. It
// reports whether any still moves.
func (el *Element) paintTableChrome(p *ui.Painter, pl *pageLayout, ox, oy float32, now time.Time, animate bool) bool {
	ch := &el.chrome
	moving := false
	for _, u := range pl.units {
		t := u.table
		tm := ch.motions[u.pres.Unit]
		if t == nil || tm == nil {
			continue
		}
		for _, a := range el.tableAreas(u) {
			el.paintArea(p, a, tm, ox, oy)
		}
		moving = el.paintMade(p, u, ox, oy, now) || moving
		moving = el.paintCarry(p, u, tm, ox, oy, now, animate) || moving
		el.paintBorder(p, u, tm, ox, oy)
		moving = el.paintGrips(p, u, tm, ox, oy, now, animate) || moving
		el.paintResize(p, u, ox, oy)
	}
	return moving
}

// paintArea draws an area as the app's create areas are: a dashed outline
// that only hints at rest and takes the accent under the pointer, over
// its tint, with a plus and what it makes; pressed, it gives a little.
func (el *Element) paintArea(p *ui.Painter, a tableArea, tm *tableMotion, ox, oy float32) {
	th := el.Theme
	size := el.chromeSize()
	scale := el.chromeScale()
	controls := el.controls()
	hover := tm.hover[a.kind].Value()
	box := a.box
	box.X += ox + a.shift
	box.Y += oy
	press := tm.press.Value()
	if d := el.chrome.drag; d == nil || d.kind != a.kind {
		press = 0
	}
	alpha := a.alpha
	draw := func() {
		p.Fill(box, components.Blend(ui.Transparent, th.Accent.Muted, 0.6*hover).Alpha(alpha), controls.Radius)
		components.DashedOutline(p, box, controls.Radius, 1, components.Blend(th.Border.Default, th.Accent.Foreground, hover).Alpha(alpha))
		ink := components.Blend(th.Text.Muted, th.Accent.Foreground, hover)
		icon := 16 * scale
		label := ui.Span{Text: "Row", Font: theme.UIFontFamily, Size: theme.TextSmall.Size(size), Color: ink.Alpha(alpha * a.label)}
		if a.kind == controlColumnArea || a.kind == controlBeside {
			label.Text = "Column"
		}
		iconBox := ui.Rect{X: box.X + (box.W-icon)/2, Y: box.Y + (box.H-icon)/2, W: icon, H: icon}
		if !a.compact {
			iconBox.X = box.X + theme.Space08.DIPs(size)
		}
		p.Clip(box, controls.Radius, func() {
			p.Icon(components.IconPlus.SVG(), iconBox, ink.Alpha(alpha*a.icon))
			if !a.compact {
				_, h := p.MeasureText(0, label)
				p.RichText(iconBox.X+icon+theme.Space06.DIPs(size), box.Y+(box.H-h)/2, 0, label)
			}
		})
	}
	if press > 0.001 {
		k := 1 - 0.02*press
		p.Transform(ui.Scaling(k, k), box.X+box.W/2, box.Y+box.H/2, draw)
		return
	}
	draw()
}

// paintMade shows a line an area made as the area it was, its tint and
// dashed outline fading into the cells.
func (el *Element) paintMade(p *ui.Painter, u *unitLayout, ox, oy float32, now time.Time) bool {
	m := el.chrome.made
	t := u.table
	if m == nil || m.table != u.pres.Unit {
		return false
	}
	n := len(t.rows)
	if m.column {
		n = len(t.columns)
	}
	if m.index >= n {
		return false
	}
	r := t.line(u, !m.column, m.index)
	r.X += ox + u.x
	r.Y += oy + u.y
	f := components.Smoothstep(1 - float32(now.Sub(m.at))/float32(madeFade))
	radius := objectRadius(el.Editor.metrics)
	p.Fill(r, el.Theme.Accent.Muted.Alpha(0.6*f), radius)
	components.DashedOutline(p, r, radius, 1, el.Theme.Accent.Foreground.Alpha(f))
	return true
}

// paintCarry shows a carry under way or landing: the frame where it would
// land, the line veiled in its place, and its copy where the pointer
// carries it, nothing set behind it.
func (el *Element) paintCarry(p *ui.Painter, u *unitLayout, tm *tableMotion, ox, oy float32, now time.Time, animate bool) bool {
	ch := &el.chrome
	t := u.table
	radius := objectRadius(el.Editor.metrics)
	if l := ch.landing; l != nil && l.table == u.pres.Unit {
		f := float32(now.Sub(l.at)) / float32(carryLand)
		n := len(t.columns)
		if l.rows {
			n = len(t.rows)
		}
		if !animate || f >= 1 || l.index >= n {
			ch.landing = nil
			return false
		}
		place := t.line(u, l.rows, l.index)
		e := components.Decelerate(max(0, f))
		at := ui.Point{X: l.from.X + (place.X-l.from.X)*e, Y: l.from.Y + (place.Y-l.from.Y)*e}
		el.paintCarried(p, u, l.rows, l.index, at, 1, ox, oy)
		return true
	}
	d := ch.drag
	rows, ok := d.carryingOK()
	if !ok || d.table != u.pres.Unit {
		return false
	}
	for k, in := range t.in.apart {
		a := tm.findApart(in.rows, in.at)
		frame := t.apart[k]
		if a == nil || !a.frame || in.room <= 0 {
			continue
		}
		// The frame takes the line's size within its room, and comes in
		// as its room opens.
		frame = t.across(frame)
		if !rows {
			frame.W = min(frame.W, d.size)
		} else {
			frame.H = min(frame.H, d.size)
		}
		frame.X += ox + u.x
		frame.Y += oy + u.y
		components.DropFrame(p, frame, radius, min(1, in.room/max(1, d.size)), el.Theme)
	}
	el.paintCarried(p, u, rows, d.from, d.at(), carryVeil, ox, oy)
	return false
}

func (tm *tableMotion) findApart(rows bool, at int) *apartRoom {
	for _, a := range tm.apart {
		if a.rows == rows && a.at == at {
			return a
		}
	}
	return nil
}

// paintCarried veils a line in its place and draws its copy with its
// corner at, in the unit: the lines between its cells, its edge, and its
// cells' text.
func (el *Element) paintCarried(p *ui.Painter, u *unitLayout, rows bool, index int, at ui.Point, veil, ox, oy float32) {
	t := u.table
	n := len(t.columns)
	if rows {
		n = len(t.rows)
	}
	if index < 0 || index >= n {
		return
	}
	ux, uy := ox+u.x, oy+u.y
	place := t.line(u, rows, index)
	place.X += ux
	place.Y += uy
	p.Fill(place, el.Theme.Surface.Page.Alpha(veil), 0)
	dx, dy := at.X+ux-place.X, at.Y+uy-place.Y
	copied := place
	copied.X += dx
	copied.Y += dy
	p.Clip(grown(copied, 1, 1), 0, func() {
		t.paintGrid(el, p, copied, 0, ux+dx, uy+dy, !rows, rows)
		for i, s := range u.surfaces {
			cc := t.cells[i]
			if cc.Row >= 0 && (rows && cc.Row == index || !rows && cc.Column == index) {
				el.paintSurface(p, s, ux+dx, uy+dy, false, 1)
			}
		}
	})
}

// paintBorder lights the column border the pointer is on or a drag holds.
func (el *Element) paintBorder(p *ui.Painter, u *unitLayout, tm *tableMotion, ox, oy float32) {
	t := u.table
	lit := tm.border.Value()
	if lit < 0.01 || tm.borderAt < 0 || tm.borderAt >= len(t.columns) {
		return
	}
	b := tableBox(u)
	b.X += ox
	b.Y += oy
	c := t.cell(doc.CellCoord{Column: tm.borderAt})
	x := ox + u.x + c.X + c.W
	p.Clip(b, u.frame, func() {
		p.Fill(ui.Rect{X: x - borderRing/2, Y: b.Y, W: borderRing, H: b.H}, el.Theme.Accent.Foreground.Alpha(0.85*lit), 0)
	})
}

// paintGrips draws the grips of the row and the column under the pointer
// and the table's corner, as the pointer comes by the table: an icon on
// nothing at rest, as the app's quiet buttons are, a tint coming in under
// the pointer and the accent as it is held; the corner's, which resizes
// rather than moves, a knob. A grip called up again glides there from
// where it was; one that was not there comes in where it is.
func (el *Element) paintGrips(p *ui.Painter, u *unitLayout, tm *tableMotion, ox, oy float32, now time.Time, animate bool) bool {
	ch := &el.chrome
	shown := tm.grips.Value()
	if shown < 0.01 {
		for k := range tm.grip {
			tm.grip[k].shown = false
		}
		return false
	}
	th := el.Theme
	controls := el.controls()
	row, column := -1, -1
	if ch.near == u.pres.Unit {
		row, column = ch.row, ch.column
	}
	rr, cr, corner := el.gripRects(u, row, column)
	moving := false
	carrying := false
	if d := ch.drag; d != nil {
		_, carrying = d.carrying()
	}
	for k, r := range [3]ui.Rect{rr, cr, corner} {
		g := &tm.grip[k]
		if r.W <= 0 {
			g.shown = false
			continue
		}
		top := r.Y + oy
		if !g.shown || top+r.H < 0 || top > el.viewport.H+el.viewport.Y {
			g.x, g.y = motion.NewSpring(r.X, now), motion.NewSpring(r.Y, now)
		} else {
			moving = follow(&g.x, r.X, now, chromeFollow, animate, false) || moving
			moving = follow(&g.y, r.Y, now, chromeFollow, animate, false) || moving
		}
		g.shown = true
		r.X, r.Y = g.x.Value()+ox, g.y.Value()+oy
		kind := [3]control{controlRow, controlColumn, controlCorner}[k]
		hover := tm.hover[kind].Value()
		held := ch.drag != nil && ch.drag.table == u.pres.Unit && ch.drag.kind == kind
		if kind == controlCorner {
			radius := min(r.W, r.H) / 2
			p.Shadow(r, radius, 0, 1, 4, 0, ui.Color{A: 255}.Alpha(0.2*shown))
			p.Fill(r, components.Blend(th.Surface.Elevated, th.Accent.Foreground, hover).Alpha(shown), radius)
			p.Stroke(r, components.Blend(th.Border.Default, th.Accent.Foreground, hover).Alpha(shown), radius, 1)
			continue
		}
		icon := components.IconGrip
		if kind == controlColumn {
			icon = components.IconGripAcross
		}
		p.Fill(r, th.GhostElement.Hover.Alpha(hover*shown), controls.Radius)
		ink := components.Blend(th.Icon.Muted, th.Icon.Default, hover)
		if held || carrying {
			ink = th.Accent.Foreground
		}
		size := gripIcon * el.chromeScale()
		p.Icon(icon.SVG(), ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, ink.Alpha(shown))
	}
	return moving
}

// paintResize shows the size a corner's drag would make the table,
// outlined and named under its far corner, and a border's drag's line.
func (el *Element) paintResize(p *ui.Painter, u *unitLayout, ox, oy float32) {
	d := el.chrome.drag
	if d == nil || d.table != u.pres.Unit || !d.moved || d.kind != controlCorner {
		return
	}
	t := u.table
	scale := el.chromeScale()
	b := tableBox(u)
	b.X += ox
	b.Y += oy
	rows, columns := len(t.rows), len(t.columns)
	r := ui.Rect{X: b.X, Y: b.Y,
		W: max(b.W/float32(columns), b.W*float32(d.columns)/float32(columns)),
		H: max(b.H/float32(rows), b.H*float32(d.rows)/float32(rows))}
	components.DashedOutline(p, r, objectRadius(el.Editor.metrics), 1.5, el.Theme.Accent.Foreground)
	el.paintChip(p, fmt.Sprintf("%d × %d", d.rows, d.columns), func(chip ui.Rect) (ui.Rect, bool) {
		// The view's right edge, in the paint's frame.
		right := ox - el.column + el.viewport.W - 8*scale
		chip.X, chip.Y = min(r.X+r.W, right)-chip.W, r.Y+r.H+4*scale
		return chip, true
	})
}

// placeForMenu readies the page's menu for where a secondary click is: on
// a row's grip or a column's, that line picked whole and its own commands;
// on a cell, the caret there, unless the selection holds it already, and
// the table's.
func (el *Element) placeForMenu(x, y float32, at time.Time) {
	ch := &el.chrome
	ch.menu = nil
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	if h := el.chromeAt(x, y); h.kind == controlRow || h.kind == controlColumn {
		rows := h.kind == controlRow
		ch.menu = columnMenu
		if rows {
			ch.menu = rowMenu
		}
		id := h.u.pres.Unit
		cr := doc.CellRange{Table: id, WholeRows: rows, WholeColumns: !rows,
			Anchor: doc.CellCoord{Row: h.at, Column: h.at}, Head: doc.CellCoord{Row: h.at, Column: h.at}}
		// A pick of lines that holds this one stays.
		if sel := el.Editor.Selection(); sel.Kind == doc.SelectCells && sel.Cells.Table == id && sel.Cells.WholeRows == rows &&
			sel.Cells.WholeColumns == !rows && between(h.at, sel.Cells.Anchor, sel.Cells.Head, rows) {
			return
		}
		el.Editor.Do(vocab.Place{Cells: &cr}, at)
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
	ch.menu = tableMenu
	sel := el.Editor.Selection()
	for _, piece := range vocab.SelectionExtent(r, sel) {
		if doc.Compare(r, piece.From, c) <= 0 && doc.Compare(r, c, piece.To) <= 0 {
			return
		}
	}
	el.Editor.Do(vocab.Place{Stop: c}, at)
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

// contextMenu is the page's menu: the table commands its secondary click
// chose, else all of them, each through the command table and offered as it would act, then
// the text's editing items.
func (el *Element) contextMenu(m *ui.Menu, services ui.Services) {
	ed := el.Editor
	labels := map[vocab.CommandID]string{}
	for _, c := range keymap.Declared(ed.Engine.Program().Catalog()) {
		labels[vocab.CommandID(c.ID)] = c.Label
	}
	shown := false
	menu := el.chrome.menu
	if menu == nil {
		// Opened from the keyboard: the table's, as the selection has it.
		menu = tableMenu
	}
	for _, group := range menu {
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

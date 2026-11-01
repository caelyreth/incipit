package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// A block of the page, and a row or a column of a table, moves as the
// sheet list's rows do. Dragging its grip lifts a copy of it under the
// pointer, only what it shows, as a list's row goes, the thing itself
// dimmed in its place.
// Where it would land the page parts, a dashed frame its size opening
// there as the one it leaves closes, and what stands around glides to make
// room. Where it would land is read from where things stood as the drag
// began, so nothing chases the pointer as the page parts; near the top or
// the bottom of the page, the page scrolls on. Letting go moves it there
// in one step of history, the copy landing from where it was let go into
// its place as the page closes behind it; letting go by its own place, or
// Escape, sets it back down where it was.

type carryKind uint8

const (
	carryBlock carryKind = iota + 1
	carryRow
	carryColumn
)

// carry follows one drag of a block, a row or a column.
type carry struct {
	kind carryKind
	// item is what it is found by as it moves: the block, or the first
	// cell of the row or the column.
	item     model.NodeID
	from, to int
	// grab is where the pointer took it, from its corner, and at where
	// the pointer carries its corner, in the page's content.
	grab, at ui.Point
	// mids are the middles of what it moves among as the drag began,
	// along the way it moves; size is its own size that way, space the
	// room the page keeps after it, and margin the room a frame keeps
	// after it, some even where blocks stand close.
	mids                []float32
	size, space, margin float32
}

// moves reports whether landing at slot moves it: either side of its own
// place is where it is.
func (c *carry) moves(slot int) bool { return slot != c.from && slot != c.from+1 }

// carryLanding is a carried thing on its way from where it was let go
// into its place.
type carryLanding struct {
	kind carryKind
	item model.NodeID
	from ui.Point
	at   time.Time
}

// carryLand is how long a landing takes.
const carryLand = components.MotionNormal

// apartKey names room the page opens where a carried thing would land, or
// closes where one was: before slot among the page's blocks, or among
// the rows or columns of the table that has cell.
type apartKey struct {
	kind carryKind
	cell model.NodeID
	slot int
}

// apartRoom is one such room: how far it opens, how far it shows open,
// and whether it is where a carry would land, which shows its frame. jump
// sets it open that far at once, as one is left behind by a move.
type apartRoom struct {
	target, shown float32
	frame         bool
	jump          float32
}

// carried is the thing under way, or landing, in the page as the layout
// sets it now.
type carried struct {
	kind  carryKind
	ok    bool
	rect  ui.Rect
	table textlayout.PlacedTable
	index int
	// ranges are the text it holds.
	ranges []model.UTF16Range
}

// findCarried finds item in layout as it stands now.
func (e *Element) findCarried(layout *textlayout.Layout, bridge *Bridge, kind carryKind, item model.NodeID) carried {
	found := carried{kind: kind}
	if layout == nil {
		return found
	}
	if kind == carryBlock {
		if bridge == nil || bridge.Controller == nil {
			return found
		}
		for _, unit := range e.unitsOf(bridge) {
			if unit.Node != item {
				continue
			}
			top, bottom, ok := layout.Extent(unit.Range)
			if !ok {
				return found
			}
			found.ok, found.ranges = true, []model.UTF16Range{unit.Range}
			found.rect = ui.Rect{Y: top, W: layout.Width(), H: bottom - top}
			if unit.Table {
				for _, placed := range layout.Tables() {
					if placed.Table.Node() == unit.Node {
						found.rect.X, found.rect.W, found.table = placed.Box.X, placed.Box.W, placed
					}
				}
			}
			return found
		}
		return found
	}
	for _, placed := range layout.Tables() {
		for r, row := range placed.Table.Rows {
			for c, cell := range row {
				if cell.Node != item || r >= len(placed.Rows) || c >= len(placed.Columns) {
					continue
				}
				found.ok, found.table = true, placed
				if kind == carryRow {
					found.index = r
					found.rect = ui.Rect{X: placed.Box.X, Y: placed.Rows[r], W: placed.Box.W, H: placed.Heights[r]}
					for _, each := range row {
						found.ranges = append(found.ranges, each.Range)
					}
				} else {
					found.index = c
					found.rect = ui.Rect{X: placed.Columns[c], Y: placed.Box.Y, W: placed.Widths[c], H: placed.Box.H}
					for _, each := range placed.Table.Rows {
						found.ranges = append(found.ranges, each[c].Range)
					}
				}
				return found
			}
		}
	}
	return found
}

// startCarry lifts the block, row or column item is, from its place among
// the others: blocks those of the page, rows and columns those of table.
func (e *Element) startCarry(bridge *Bridge, kind carryKind, item model.NodeID, from int, moved ui.Point) {
	found := e.findCarried(e.current, bridge, kind, item)
	if !found.ok {
		return
	}
	pointer, _ := e.pointerPoint()
	press := ui.Point{X: pointer.X - moved.X, Y: pointer.Y - moved.Y}
	next := &carry{
		kind: kind, item: item, from: from, to: from,
		grab: ui.Point{X: press.X - found.rect.X, Y: press.Y - found.rect.Y},
		at:   ui.Point{X: found.rect.X, Y: found.rect.Y},
	}
	switch kind {
	case carryBlock:
		units := e.placedUnits(bridge)
		for _, unit := range units {
			next.mids = append(next.mids, (unit.top+unit.bottom)/2)
		}
		next.size = found.rect.H
		if from > 0 {
			next.space = units[from].top - units[from-1].bottom
		} else if len(units) > 1 {
			next.space = units[1].top - units[0].bottom
		}
		next.margin = max(next.space, e.Prose.LineHeight()/3)
	case carryRow:
		for r := range found.table.Rows {
			next.mids = append(next.mids, found.table.Rows[r]+found.table.Heights[r]/2)
		}
		next.size = found.rect.H
	case carryColumn:
		for c := range found.table.Columns {
			next.mids = append(next.mids, found.table.Columns[c]+found.table.Widths[c]/2)
		}
		next.size = found.rect.W
	}
	e.carry = next
}

// followCarry carries what a drag holds with the pointer, and opens the
// page where it would land.
func (e *Element) followCarry(box ui.Element, animate bool) {
	if c := e.carry; c != nil {
		// The page scrolls on by its edges as the pointer comes to them.
		x, y, _ := box.PointerPosition()
		e.pointerAt = ui.Point{X: x, Y: y}
		pointer, _ := e.pointerPoint()
		middle := float32(0)
		if c.kind == carryColumn {
			c.at.X = pointer.X - c.grab.X
			middle = c.at.X + c.size/2
		} else {
			c.at.Y = pointer.Y - c.grab.Y
			middle = c.at.Y + c.size/2
		}
		c.to = 0
		for _, mid := range c.mids {
			if mid < middle {
				c.to++
			}
		}
		for key, room := range e.apart {
			if room.frame && (key.kind != c.kind || key.slot != c.to) {
				room.target, room.frame = 0, false
			}
		}
		if c.moves(c.to) {
			key := apartKey{kind: c.kind, cell: c.cell(), slot: c.to}
			room := e.apartRoom(key)
			room.target, room.frame = c.size+c.margin, true
		}
	}
	for key, room := range e.apart {
		if room.jump > 0 {
			box.Jump(key, room.jump)
			room.jump = 0
		}
		room.shown = room.target
		if animate {
			room.shown = components.Follow(box, key, room.target, components.MotionQuick)
		}
		if room.target == 0 && room.shown < 0.5 {
			delete(e.apart, key)
		}
	}
}

// cell is the cell a row's or column's carry finds its table by.
func (c *carry) cell() model.NodeID {
	if c.kind == carryBlock {
		return 0
	}
	return c.item
}

func (e *Element) apartRoom(key apartKey) *apartRoom {
	if e.apart == nil {
		e.apart = map[apartKey]*apartRoom{}
	}
	room := e.apart[key]
	if room == nil {
		room = &apartRoom{}
		e.apart[key] = room
	}
	return room
}

// dropCarry lets what a drag holds go: where it would land, in one step of
// history, unless back is set or it would land where it is. Either way
// its copy lands from where it was let go.
func (e *Element) dropCarry(bridge *Bridge, back bool, now time.Time) {
	c := e.carry
	if c == nil {
		return
	}
	e.carry = nil
	e.focusPending = true
	e.landing = &carryLanding{kind: c.kind, item: c.item, from: c.at, at: now}
	for _, room := range e.apart {
		if room.frame {
			// The copy lands in the frame's room: it goes at once.
			room.target, room.shown, room.frame, room.jump = 0, 0, false, 0
		}
	}
	if back || !c.moves(c.to) || bridge == nil || bridge.Controller == nil {
		return
	}
	var err error
	switch c.kind {
	case carryBlock:
		units := e.unitsOf(bridge)
		before := model.NodeID(0)
		if c.to < len(units) {
			before = units[c.to].Node
		}
		_, err = bridge.Controller.MoveBlock(c.item, before)
	case carryRow:
		_, err = bridge.Controller.MoveTableRow(c.item, c.from, carryIndex(c.from, c.to))
	case carryColumn:
		_, err = bridge.Controller.MoveTableColumn(c.item, c.from, carryIndex(c.from, c.to))
	}
	if err != nil {
		return
	}
	// The room it stood in closes behind it, before what followed it.
	left := c.from
	if c.to < c.from {
		left = c.from + 1
	}
	room := e.apartRoom(apartKey{kind: c.kind, cell: c.cell(), slot: left})
	room.target, room.frame = 0, false
	room.jump, room.shown = c.size+c.space, c.size+c.space
}

// carryIndex is where a row or column from goes, landing before slot.
func carryIndex(from, slot int) int {
	if slot > from {
		return slot - 1
	}
	return slot
}

// unitsOf are the page's blocks, as its text has them now.
func (e *Element) unitsOf(bridge *Bridge) []editor.Unit {
	if text := bridge.Controller.Text(); text != e.blocks.text || e.blocks.units == nil {
		e.blocks.text, e.blocks.units = text, bridge.Controller.Units()
	}
	return e.blocks.units
}

// apartFlow is flow with the room the page opens between its blocks.
func (e *Element) apartFlow(flow *surface.Flow, bridge *Bridge) *surface.Flow {
	if len(e.apart) == 0 || bridge == nil || bridge.Controller == nil {
		return flow
	}
	units := e.unitsOf(bridge)
	var rooms []surface.Room
	for key, room := range e.apart {
		if key.kind != carryBlock || room.shown < 0.5 {
			continue
		}
		at := model.UTF16Offset(-1)
		if key.slot < len(units) {
			at = units[key.slot].Range.Start
		}
		rooms = append(rooms, surface.Room{At: at, Height: room.shown})
	}
	return flow.WithRooms(rooms)
}

// tableApart sets the room the page opens between table's rows and its
// columns.
func (e *Element) tableApart(table surface.Table, room *surface.TableRoom) {
	room.RowsApart, room.ColumnsApart = nil, nil
	for key, open := range e.apart {
		if key.kind == carryBlock || open.shown < 0.5 || !tableHas(table, key.cell) {
			continue
		}
		rooms := &room.RowsApart
		count := len(table.Rows)
		if key.kind == carryColumn {
			rooms, count = &room.ColumnsApart, table.Columns()
		}
		if *rooms == nil {
			*rooms = make([]float32, count+1)
		}
		if key.slot <= count {
			(*rooms)[key.slot] += open.shown
		}
	}
}

func tableHas(table surface.Table, cell model.NodeID) bool {
	for _, row := range table.Rows {
		for _, each := range row {
			if each.Node == cell {
				return true
			}
		}
	}
	return false
}

// carryShown is how a carry under way or a landing shows this frame: the
// thing where it stands, its copy's corner and how hidden the thing; the
// frames where it would land, by their room.
type carryShown struct {
	thing  carried
	at     ui.Point
	veil   float32
	frames []ui.Rect
	// size and space size a frame within its room.
	size, space float32
}

// showCarry is how the carry or landing shows over layout, now.
func (e *Element) showCarry(layout *textlayout.Layout, bridge *Bridge, now time.Time, animate bool) (*carryShown, bool) {
	if l := e.landing; l != nil {
		t := float32(now.Sub(l.at)) / float32(carryLand)
		thing := e.findCarried(layout, bridge, l.kind, l.item)
		if !animate || t >= 1 || !thing.ok {
			e.landing = nil
			return nil, false
		}
		p := components.Decelerate(max(0, t))
		at := ui.Point{
			X: l.from.X + (thing.rect.X-l.from.X)*p,
			Y: l.from.Y + (thing.rect.Y-l.from.Y)*p,
		}
		return &carryShown{thing: thing, at: at, veil: 1}, true
	}
	c := e.carry
	if c == nil {
		return nil, false
	}
	thing := e.findCarried(layout, bridge, c.kind, c.item)
	if !thing.ok {
		return nil, false
	}
	shown := &carryShown{thing: thing, at: c.at, veil: carryVeil, size: c.size, space: c.margin}
	for key, room := range e.apart {
		if !room.frame || key.kind != c.kind {
			continue
		}
		switch c.kind {
		case carryBlock:
			for _, placed := range layout.Rooms() {
				if want := blockSlotAt(e.unitsOf(bridge), key.slot); placed.At == want {
					shown.frames = append(shown.frames, placed.Box)
				}
			}
		case carryRow:
			if key.slot < len(thing.table.RowsApart) {
				shown.frames = append(shown.frames, thing.table.RowsApart[key.slot])
			}
		case carryColumn:
			if key.slot < len(thing.table.ColumnsApart) {
				shown.frames = append(shown.frames, thing.table.ColumnsApart[key.slot])
			}
		}
	}
	return shown, false
}

// carryVeil is how far a carried thing is hidden in its place.
const carryVeil = 0.6

// blockSlotAt is where room before slot among units opens.
func blockSlotAt(units []editor.Unit, slot int) model.UTF16Offset {
	if slot < len(units) {
		return units[slot].Range.Start
	}
	return -1
}

// drawCarry shows a carry under way or a landing: the frame where it
// would land, the thing veiled in its place, and its copy where the
// pointer carries it, nothing set behind it, as a list's row is carried.
func drawCarry(p *ui.Painter, rect ui.Rect, state *State) {
	shown := state.Carry
	if shown == nil {
		return
	}
	offset := ui.Point{X: rect.X, Y: rect.Y + state.TopInset - state.ScrollY}
	radius := figureRadius(state)
	screen := func(r ui.Rect) ui.Rect {
		r.X += offset.X
		r.Y += offset.Y
		return r
	}
	thing := shown.thing
	for _, frame := range shown.frames {
		// The frame takes the thing's size within its room, the room the
		// page keeps beside it below it, and comes in as its room opens.
		open, extent := frame, frame.H
		if thing.kind == carryColumn {
			open.W, extent = min(frame.W, shown.size), frame.W
		} else {
			open.H = min(frame.H, shown.size)
		}
		reveal := min(1, extent/max(1, shown.size+shown.space))
		components.DropFrame(p, screen(open), radius, reveal, state.Theme)
	}
	place := screen(thing.rect)
	p.Fill(place, state.Theme.Surface.Page.Alpha(shown.veil), 0)
	d := ui.Point{X: shown.at.X - thing.rect.X, Y: shown.at.Y - thing.rect.Y}
	copied := place
	copied.X += d.X
	copied.Y += d.Y
	// A table's cells keep within what is carried of them.
	p.Clip(grown(copied, 1, 1), 0, func() {
		drawCarriedTable(p, state, thing, d, offset)
		for _, paragraph := range state.Layout.Paragraphs() {
			start := model.UTF16Offset(paragraph.StartUTF16())
			if !carriedHolds(thing.ranges, start) {
				continue
			}
			p.TextLayout(
				paragraph.Layout(),
				offset.X+paragraph.Left()+d.X,
				offset.Y+paragraph.Top()+d.Y,
				state.Theme.Prose.Text,
			)
		}
	})
}

func carriedHolds(ranges []model.UTF16Range, at model.UTF16Offset) bool {
	for _, r := range ranges {
		if r.Start <= at && at <= r.End {
			return true
		}
	}
	return false
}

// drawCarriedTable draws the lines of what of a table a copy carries, as
// the table draws them: its edge, and a row's between its cells, a
// column's between its rows, a table's all.
func drawCarriedTable(p *ui.Painter, state *State, thing carried, d, offset ui.Point) {
	placed := thing.table
	if placed.Table.Rows == nil {
		return
	}
	line := state.Theme.Border.Default
	at := func(r ui.Rect) ui.Rect {
		r.X += offset.X + d.X
		r.Y += offset.Y + d.Y
		return r
	}
	box := thing.rect
	header := placed.Table.Header
	defer p.Stroke(at(box), line, 0, 1)
	switch thing.kind {
	case carryRow:
		if header && thing.index == 0 {
			p.Fill(at(box), state.Theme.Surface.Inset, 0)
		}
		for c := 1; c < len(placed.Columns); c++ {
			p.Fill(at(ui.Rect{X: placed.Columns[c], Y: box.Y, W: 1, H: box.H}), line, 0)
		}
	case carryColumn:
		if header && len(placed.Rows) > 0 {
			p.Fill(at(ui.Rect{X: box.X, Y: placed.Rows[0], W: box.W, H: placed.Heights[0]}), state.Theme.Surface.Inset, 0)
		}
		for r := 1; r < len(placed.Rows); r++ {
			p.Fill(at(ui.Rect{X: box.X, Y: placed.Rows[r], W: box.W, H: 1}), line, 0)
		}
	case carryBlock:
		if header && len(placed.Rows) > 0 {
			p.Fill(at(ui.Rect{X: box.X, Y: placed.Rows[0], W: box.W, H: placed.Heights[0]}), state.Theme.Surface.Inset, 0)
		}
		for r := 1; r < len(placed.Rows); r++ {
			p.Fill(at(ui.Rect{X: box.X, Y: placed.Rows[r], W: box.W, H: 1}), line, 0)
		}
		for c := 1; c < len(placed.Columns); c++ {
			p.Fill(at(ui.Rect{X: placed.Columns[c], Y: box.Y, W: 1, H: box.H}), line, 0)
		}
	}
}

package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// Every block of the page moves by a grip in the gutter before it, as a
// table's rows and columns move by theirs: the pointer by a block shows
// its grip by its first line, a table's at its corner, where its rows'
// grips and its columns' meet. A click selects the block whole; a drag
// carries it among the others (carry.go).

// blockUI is what the page keeps of its blocks across frames.
type blockUI struct {
	// text is the page's text units were read for, and units the page's
	// blocks as they move.
	text  string
	units []editor.Unit
	// hovered is the block the pointer is by, its place in units, and
	// shown how far its grip has come in.
	hovered int
	shown   float32
	// held is the block whose grip a press holds, while holding: it stays
	// the block until the press lets go though the pointer leaves it.
	held    int
	holding bool
}

// placedUnit is a block where the last frame set it.
type placedUnit struct {
	unit        editor.Unit
	top, bottom float32
}

// placedUnits are the page's blocks where the last frame set them.
func (e *Element) placedUnits(bridge *Bridge) []placedUnit {
	units := e.unitsOf(bridge)
	placed := make([]placedUnit, 0, len(units))
	for _, unit := range units {
		if top, bottom, ok := e.current.Extent(unit.Range); ok {
			placed = append(placed, placedUnit{unit: unit, top: top, bottom: bottom})
		}
	}
	return placed
}

// followBlocks finds the block the pointer is by, as the last frame set
// the page out, and brings its grip in.
func (e *Element) followBlocks(box ui.Element, bridge *Bridge, animate bool) {
	e.blocks.hovered = -1
	if e.current == nil || bridge == nil || bridge.Controller == nil {
		return
	}
	point, inside := e.pointerPoint()
	if e.blocks.holding {
		e.blocks.hovered = e.blocks.held
	} else if inside && e.pointer == pointerIdle && e.carry == nil && e.tables.drag == nil && e.tables.sizing == nil {
		gutter := tableReach * e.uiScale()
		for index, unit := range e.placedUnits(bridge) {
			left, right := -gutter, e.current.Width()
			if unit.unit.Table {
				if box, ok := e.tableBox(unit.unit.Node); ok {
					left, right = box.X-gutter, box.X+box.W
				}
			}
			if point.X >= left && point.X <= right && point.Y >= unit.top-gutter/2 && point.Y < unit.bottom+gutter/2 {
				e.blocks.hovered = index
			}
		}
	}
	target := float32(0)
	if e.blocks.hovered >= 0 {
		target = 1
	}
	e.blocks.shown = target
	if animate {
		e.blocks.shown = components.Follow(box, "block-grip", target, components.MotionNormal)
	}
}

// tableBox is the grid of the table node starts, as the last frame set
// it out.
func (e *Element) tableBox(node model.NodeID) (ui.Rect, bool) {
	for _, table := range e.current.Tables() {
		if table.Table.Node() == node {
			return table.Box, true
		}
	}
	return ui.Rect{}, false
}

// blockControls places the grip of the block the pointer is by.
func (e *Element) blockControls(c *ui.Context, bridge *Bridge, animate bool) {
	if e.blocks.hovered < 0 || e.blocks.shown < 0.01 {
		return
	}
	units := e.placedUnits(bridge)
	if e.blocks.hovered >= len(units) {
		return
	}
	unit := units[e.blocks.hovered]
	scale := e.uiScale()
	long, short, off := gripLong*scale, gripShort*scale, gripOff*scale
	r := ui.Rect{
		X: -off - short,
		Y: unit.top + (min(e.Prose.LineHeight(), unit.bottom-unit.top)-long)/2,
		W: short, H: long,
	}
	label := "Move block"
	if unit.unit.Table {
		// A table's grip is at its corner, where its rows' grips and its
		// columns' meet.
		box, _ := e.tableBox(unit.unit.Node)
		r = ui.Rect{X: box.X - off - short, Y: box.Y - off - short, W: short, H: short}
		label = "Move table"
	}
	handle := e.grip(c, "block-grip", label, r, e.blocks.shown, components.IconGrip, animate)
	// A grip stays held until its press, and what it carries, lets go.
	e.blocks.held = e.blocks.hovered
	e.blocks.holding = handle.Pressed() || e.carry != nil && e.carry.kind == carryBlock
	e.followBlockGrip(handle, bridge, units, e.blocks.hovered, c.Now())
}

// followBlockGrip follows a press on a block's grip: a click selects the
// block whole, and a drag carries it among the others.
func (e *Element) followBlockGrip(handle ui.Element, bridge *Bridge, units []placedUnit, from int, now time.Time) {
	unit := units[from].unit
	if dx, dy, dragging := handle.Dragged(); dragging && e.carry == nil {
		e.startCarry(bridge, carryBlock, unit.Node, from, ui.Point{X: dx, Y: dy})
	}
	if _, _, ended := handle.DragEnded(); ended {
		e.dropCarry(bridge, false, now)
		return
	}
	if !handle.Clicked() || e.carry != nil {
		return
	}
	e.focusPending = true
	if unit.Table {
		_ = bridge.Controller.SelectTable(unit.Node)
		return
	}
	_ = bridge.Controller.SelectProjected(int(unit.Range.Start), int(unit.Range.End))
}

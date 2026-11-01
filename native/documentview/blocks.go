package documentview

import (
	"fmt"
	"math"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/components"
	"github.com/egoist/mygo/ui"
)

// Every block of the page moves by a grip in the gutter before it, as a
// table's rows and columns move by theirs: the pointer by a block shows
// its grip by its first line, a table's at its corner, where its rows'
// grips and its columns' meet. A click selects the block whole; a drag
// carries it, its band following the pointer and the drop line showing
// where between the others it would stand, and letting go moves it there,
// one step of history.

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
	drag    *blockDrag
	// held is the block whose grip a press holds, while holding: it stays
	// the block until the press lets go though the pointer leaves it.
	held    int
	holding bool
}

// blockDrag follows a block's grip while it is dragged.
type blockDrag struct {
	from int
	// moved is how far the pointer has gone down, and to the place among
	// the blocks the block would stand before, len for the page's end.
	moved float32
	to    int
}

// placedUnit is a block where the last frame set it.
type placedUnit struct {
	unit        editor.Unit
	top, bottom float32
}

// placedUnits are the page's blocks where the last frame set them.
func (e *Element) placedUnits(bridge *Bridge) []placedUnit {
	if text := bridge.Controller.Text(); text != e.blocks.text || e.blocks.units == nil {
		e.blocks.text, e.blocks.units = text, bridge.Controller.Units()
	}
	placed := make([]placedUnit, 0, len(e.blocks.units))
	for _, unit := range e.blocks.units {
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
	if e.blocks.drag != nil {
		e.blocks.hovered = e.blocks.drag.from
	} else if e.blocks.holding {
		e.blocks.hovered = e.blocks.held
	} else if inside && e.pointer == pointerIdle && e.tables.drag == nil && e.tables.sizing == nil {
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
	offset := e.TopInset - e.visibleScroll()
	r := ui.Rect{
		X: -off - short,
		Y: unit.top + (min(e.Prose.LineHeight(), unit.bottom-unit.top)-long)/2 + offset,
		W: short, H: long,
	}
	label := "Move block"
	if unit.unit.Table {
		// A table's grip is at its corner, where its rows' grips and its
		// columns' meet.
		box, _ := e.tableBox(unit.unit.Node)
		r = ui.Rect{X: box.X - off - short, Y: box.Y - off - short + offset, W: short, H: short}
		label = "Move table"
	}
	handle := e.grip(c, fmt.Sprintf("block-%d", unit.unit.Node), label, r, e.blocks.shown, ui.CursorGrab, animate)
	e.blocks.held, e.blocks.holding = e.blocks.hovered, handle.Pressed()
	e.followBlockGrip(handle, bridge, units, e.blocks.hovered)
}

// followBlockGrip follows a press on a block's grip: a click selects the
// block whole, and a drag moves it among the others.
func (e *Element) followBlockGrip(handle ui.Element, bridge *Bridge, units []placedUnit, from int) {
	unit := units[from].unit
	if _, dy, dragging := handle.Dragged(); dragging {
		drag := e.blocks.drag
		if drag == nil || drag.from != from {
			drag = &blockDrag{from: from, to: from}
			e.blocks.drag = drag
		}
		drag.moved += dy
		middle := units[from].top + e.Prose.LineHeight()/2 + drag.moved
		drag.to = nearestEdge(units, middle)
	}
	if _, _, ended := handle.DragEnded(); ended {
		drag := e.blocks.drag
		e.blocks.drag = nil
		e.focusPending = true
		if drag == nil || drag.to == drag.from || drag.to == drag.from+1 {
			return
		}
		before := model.NodeID(0)
		if drag.to < len(units) {
			before = units[drag.to].unit.Node
		}
		_, _ = bridge.Controller.MoveBlock(unit.Node, before)
		return
	}
	if !handle.Clicked() || e.blocks.drag != nil {
		return
	}
	e.focusPending = true
	if unit.Table {
		_ = bridge.Controller.SelectTable(unit.Node)
		return
	}
	_ = bridge.Controller.SelectProjected(int(unit.Range.Start), int(unit.Range.End))
}

// edgeY is where the edge before the block at index stands, between it
// and the one before, len for below the last.
func edgeY(units []placedUnit, index int) float32 {
	switch {
	case index == 0:
		return units[0].top
	case index >= len(units):
		return units[len(units)-1].bottom
	}
	return (units[index-1].bottom + units[index].top) / 2
}

// nearestEdge is the edge between blocks nearest y.
func nearestEdge(units []placedUnit, y float32) int {
	best, distance := 0, float32(math.Inf(1))
	for index := 0; index <= len(units); index++ {
		if d := abs(edgeY(units, index) - y); d < distance {
			best, distance = index, d
		}
	}
	return best
}

// blockDropLine is where a block's drag would put it, in the page's
// content, if one is under way.
func (e *Element) blockDropLine(bridge *Bridge) (float32, bool) {
	drag := e.blocks.drag
	if drag == nil || e.current == nil {
		return 0, false
	}
	units := e.placedUnits(bridge)
	if drag.from >= len(units) {
		return 0, false
	}
	return edgeY(units, drag.to), true
}

// blockBand is the dragged block where the pointer carries it, in the
// page's content, if one is.
func (e *Element) blockBand(bridge *Bridge) (ui.Rect, bool) {
	drag := e.blocks.drag
	if drag == nil || e.current == nil {
		return ui.Rect{}, false
	}
	units := e.placedUnits(bridge)
	if drag.from >= len(units) {
		return ui.Rect{}, false
	}
	unit := units[drag.from]
	band := ui.Rect{Y: unit.top + drag.moved, W: e.current.Width(), H: unit.bottom - unit.top}
	if unit.unit.Table {
		if box, ok := e.tableBox(unit.unit.Node); ok {
			band.X, band.W = box.X, box.W
		}
	}
	return band, true
}

// drawBlockDrag shows the block a drag carries, as a band under the
// pointer.
func drawBlockDrag(p *ui.Painter, rect ui.Rect, state *State) {
	band, ok := state.BlockBand, state.BlockDragging
	if !ok {
		return
	}
	band.X += rect.X
	band.Y += rect.Y + state.TopInset - state.ScrollY
	radius := figureRadius(state)
	p.Fill(band, state.Theme.Accent.Muted.Alpha(0.5), radius)
	p.Stroke(band, state.Theme.Accent.Foreground.Alpha(0.85), radius, figureRingWidth)
}

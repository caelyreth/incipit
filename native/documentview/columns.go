package documentview

import (
	"slices"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// A column's width is set by the border after it. The pointer on a border
// between two cells shows the resize cursor and lights the border; a drag
// moves it, the two columns beside it trading width so the rest of the
// table stays where it is, and the table's own edge sizes its last column,
// the others kept as they show. Near the width a column's text asks, the
// border snaps to it. Letting go keeps the widths in one step of history,
// Escape puts them back, and a double-click on a border gives the columns
// it sets back their widths as their text asks, the edge every column's.
// A press on a border is the border's: the caret stays where it was.

// columnGrip is how near a border, in DIPs each side, the pointer takes it.
const columnGrip = 4

// columnSizing follows one drag of a column's border.
type columnSizing struct {
	table model.NodeID
	// column is the column the border ends, and edge says it is the
	// table's own.
	column int
	edge   bool
	// at is where the press was, room the page's column the shares are
	// of, from each column's width as it showed and natural as its text
	// asks; widths are the shares the drag sets.
	at      float32
	room    float32
	from    []float32
	natural []float32
	widths  []float32
}

// columnBorder is the border the pointer is on, and how lit it is.
type columnBorder struct {
	table  model.NodeID
	column int
	on     bool
	shown  float32
}

// borderAt is the table and the column whose border is at a point of the
// element, as the last frame set it out: only a border that shows, in the
// room its table shows in.
func (e *Element) borderAt(x, y float32) (textlayout.PlacedTable, int, bool) {
	if e.current == nil {
		return textlayout.PlacedTable{}, 0, false
	}
	return borderAt(e.current.Tables(), e.contentPoint(x, y), columnGrip*e.uiScale())
}

func borderAt(tables []textlayout.PlacedTable, point ui.Point, grip float32) (textlayout.PlacedTable, int, bool) {
	for _, placed := range tables {
		box := placed.Box
		if point.Y < box.Y || point.Y >= box.Y+box.H || point.X < box.X-grip || point.X > box.X+box.W+grip {
			continue
		}
		best, nearest := -1, grip
		for column := range placed.Columns {
			edge := placed.Columns[column] + placed.Widths[column]
			if edge < box.X+1 || edge > box.X+box.W+1 {
				continue
			}
			if d := abs(point.X - edge); d <= nearest {
				best, nearest = column, d
			}
		}
		if best >= 0 {
			return placed, best, true
		}
	}
	return textlayout.PlacedTable{}, 0, false
}

// followBorder lights the border the pointer is on and shows the resize
// cursor over it, while nothing else holds the pointer.
func (e *Element) followBorder(box ui.Element, point ui.Point, free bool) {
	border := &e.tables.border
	border.on = false
	if sizing := e.tables.sizing; sizing != nil {
		border.table, border.column, border.on = sizing.table, sizing.column, true
	} else if free && e.current != nil {
		if placed, column, ok := borderAt(e.current.Tables(), point, columnGrip*e.uiScale()); ok {
			border.table, border.column, border.on = placed.Table.Node(), column, true
		}
	}
	target := float32(0)
	if border.on {
		target = 1
		box.Cursor(ui.CursorResizeEW)
	}
	border.shown = target
	if e.Animate {
		border.shown = components.Follow(box, "column-border", target, components.MotionNormal)
	}
}

// pressBorder takes a press on a column's border: a double-click gives the
// columns it sets back their own widths, and a press starts a drag.
func (e *Element) pressBorder(event ui.InputEvent) bool {
	if event.Button != 0 || event.Mods&ui.Shift != 0 {
		return false
	}
	placed, column, ok := e.borderAt(event.X, event.Y)
	if !ok {
		return false
	}
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return false
	}
	last := column == len(placed.Widths)-1
	if event.Clicks >= 2 {
		e.tables.sizing = nil
		widths := slices.Clone(placed.Table.Widths)
		for index := range widths {
			if last || index == column || index == column+1 {
				widths[index] = 0
			}
		}
		_, _ = bridge.Controller.ShapeTable(placed.Table.Node(), func(table *editor.Table) { table.Widths = widths })
		return true
	}
	e.tables.sizing = &columnSizing{
		table:   placed.Table.Node(),
		column:  column,
		edge:    last,
		at:      event.X,
		room:    e.current.Width(),
		from:    slices.Clone(placed.Widths),
		natural: slices.Clone(placed.Natural),
		widths:  slices.Clone(placed.Table.Widths),
	}
	return true
}

// dragBorder follows the pointer with the border a drag holds.
func (e *Element) dragBorder(x float32) {
	sizing := e.tables.sizing
	least := max(textlayout.LeastColumn, editor.LeastColumnWidth*sizing.room)
	snap := func(column int, width float32) float32 {
		if natural := sizing.natural[column]; abs(width-natural) <= figureSnap {
			return natural
		}
		return width
	}
	moved := x - sizing.at
	column := sizing.column
	if sizing.edge {
		// The edge sizes the last column, the others kept as they show.
		for index, width := range sizing.from {
			sizing.widths[index] = width / sizing.room
		}
		width := snap(column, max(least, min(sizing.room, sizing.from[column]+moved)))
		sizing.widths[column] = width / sizing.room
		return
	}
	// A border between two columns trades width between them.
	both := sizing.from[column] + sizing.from[column+1]
	width := snap(column, max(least, min(both-least, sizing.from[column]+moved)))
	if other := snap(column+1, both-width); other != both-width && other >= least {
		width = both - other
	}
	sizing.widths[column] = width / sizing.room
	sizing.widths[column+1] = (both - width) / sizing.room
}

// finishBorder keeps the widths a drag set, in one step of history.
func (e *Element) finishBorder() {
	sizing := e.tables.sizing
	e.tables.sizing = nil
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return
	}
	_, _ = bridge.Controller.ShapeTable(sizing.table, func(table *editor.Table) { table.Widths = sizing.widths })
}

// drawBorder lights the border the pointer is on or a drag holds.
func drawBorder(p *ui.Painter, state *State, placed textlayout.PlacedTable, box ui.Rect, dx float32) {
	border := state.TableBorder
	if border.shown < 0.01 || border.table != placed.Table.Node() || border.column >= len(placed.Columns) {
		return
	}
	x := placed.Columns[border.column] + placed.Widths[border.column] + dx
	line := ui.Rect{X: x - figureRingWidth/2, Y: box.Y, W: figureRingWidth, H: box.H}
	p.Clip(box, figureRadius(state), func() {
		p.Fill(line, state.Theme.Accent.Foreground.Alpha(0.85*border.shown), 0)
	})
}

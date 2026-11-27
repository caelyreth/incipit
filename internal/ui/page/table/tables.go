package table

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/egoist/mygo/ui"
)

// A table is its cells set out in a grid (vocab.Grid): each cell one of
// its surfaces, laid at its column's width in a little room of its own,
// each row as tall as its tallest cell. Its columns take the text's width
// when their text fits on a line, the room to spare shared evenly; a table
// asking more wraps its long columns and keeps its short ones on a line. A
// table wider than the text spreads into the margins, as far each side,
// and one wider still shows as much as they hold, scrolled across within
// them. Set apart from the text around it (GroupTable's room), it reads as
// one thing on the page.
const (
	// cellPadX and cellPadY are a cell's room around its text, as shares
	// of the body's line.
	cellPadX = 0.4
	cellPadY = 0.2
	// leastColumn is the narrowest a column is set, in DIPs, so a table of
	// many columns still shows each; readColumn the least a column that
	// wraps keeps, in the body's lines, its longest word if wider.
	leastColumn = 32
	readColumn  = 4
)

// Layout is a table's grid as it is laid out, in its unit.
type Layout struct {
	// columns are each column's left edge from the grid's and widths their
	// widths; rows are each row's top in the unit and heights theirs.
	columns, widths, rows, heights []float32
	// cells are each surface's cell, a row of -1 for one outside the grid.
	cells []doc.CellCoord
	// base and baseY are each surface's place with the grid unscrolled.
	base, baseY []float32
	header      bool
	// left is the grid's left edge in the unit unscrolled; overflow how far
	// it scrolls across what it shows (the unit's box), and scroll how far
	// it is scrolled as drawn, past an end while it pulls elastic there.
	left, overflow, scroll float32
	// in is what the page asked of the layout, and below, rowGap,
	// columnGap and apart the rooms it gave, in the unit, those across
	// unscrolled: under the foot, between two rows, between two columns,
	// and each that a carry opens, as in.apart has them.
	in                       In
	below, rowGap, columnGap ui.Rect
	apart                    [maxApart]ui.Rect
	// natural are each row's height as its text sets it, before the
	// lines' motion; gone the rooms of the lines that left, as they close,
	// rows then columns.
	natural []float32
	gone    [2][]goneRoom
}

// goneRoom is the room a line that left still has, in the unit, those
// across unscrolled: drawn as it was kept, as strongly as alpha.
type goneRoom struct {
	room  ui.Rect
	kept  keptLine
	alpha float32
}

// In is what the page asks of a table's layout while its controls
// show, each as far as it has come: the room an area opens under the foot
// (below), between two rows before row, and between two columns before
// column; the rooms a carry opens before a line; the lines coming in; and
// a column's width as its border's Drag sets it. Its zero value asks
// nothing.
type In struct {
	below               float32
	row, column         int
	rowRoom, columnRoom float32
	apart               [maxApart]lineRoom
	// lines are the lines on their way, rows then columns (table_motion.go);
	// nil while they stand.
	lines *lineMotion
	sized int
	width float32
}

// lineMotion is how a table's lines are drawn on their way, rows then
// columns, by line: the extent each is drawn at (a row's height, a
// column's share of its width), -1 as laid out; how far each stands from
// its place; and the lines that left, closing.
type lineMotion struct {
	extent, offset [2][]float32
	gone           [2][]lineGone
}

// lineGone is a line that left: after line after (-1 before the first),
// its room, as strongly as alpha, drawn as kept; or, where a line took its
// place, within line cover at lead from its start.
type lineGone struct {
	after, cover int
	room, lead   float32
	alpha        float32
	kept         keptLine
}

// keptLine is a line as it was last laid out: its table's unit then, and
// its number there.
type keptLine struct {
	u     *Unit
	index int
}

// extentOf is how a line is drawn: as laid out, or as its motion has it.
func (lm *lineMotion) extentOf(axis, i int) (float32, bool) {
	if lm == nil || i >= len(lm.extent[axis]) || lm.extent[axis][i] < 0 {
		return 0, false
	}
	return lm.extent[axis][i], true
}

func (lm *lineMotion) offsetOf(axis, i int) float32 {
	if lm == nil || i >= len(lm.offset[axis]) {
		return 0
	}
	return lm.offset[axis][i]
}

// goneAfter are the lines that left after line i, uncovered.
func (lm *lineMotion) goneAfter(axis, i int) []lineGone {
	if lm == nil {
		return nil
	}
	var out []lineGone
	for _, g := range lm.gone[axis] {
		if g.after == i && g.cover < 0 {
			out = append(out, g)
		}
	}
	return out
}

// maxApart is how many rooms a carry has open at once in a table: the one
// it would land in and those it left, closing.
const maxApart = 3

// lineRoom is room before line at, among the rows or the columns.
type lineRoom struct {
	rows bool
	at   int
	room float32
}

// apartAt is the room the carry opens before line at.
func (in In) apartAt(rows bool, at int) float32 {
	room := float32(0)
	for _, a := range in.apart {
		if a.rows == rows && a.at == at {
			room += a.room
		}
	}
	return room
}

// share is the share of its width a column is drawn at.
func (in In) share(c int) float32 {
	if e, ok := in.lines.extentOf(1, c); ok {
		return e
	}
	return 1
}

// cell is a cell's box in its unit, as the grid is scrolled.
func (t *Layout) cell(cc doc.CellCoord) ui.Rect {
	return ui.Rect{X: t.left - t.scroll + t.columns[cc.Column], Y: t.rows[cc.Row], W: t.widths[cc.Column], H: t.heights[cc.Row]}
}

// line is a row's box across what the table shows, or a column's down
// it, in its unit, as the grid is scrolled.
func (t *Layout) line(u *Unit, rows bool, i int) ui.Rect {
	if rows {
		return ui.Rect{X: u.Box.X, Y: t.rows[i], W: u.Box.W, H: t.heights[i]}
	}
	return ui.Rect{X: t.left - t.scroll + t.columns[i], Y: u.Box.Y, W: t.widths[i], H: u.Box.H}
}

// rowAt and columnAt are the row and the column at a place across the
// unit, -1 past the grid.
func (t *Layout) rowAt(y float32) int {
	for r := range t.rows {
		if top := t.rows[r]; y >= top && y < top+t.heights[r] {
			return r
		}
	}
	return -1
}

func (t *Layout) columnAt(x float32) int {
	for c := range t.columns {
		if left := t.left - t.scroll + t.columns[c]; x >= left && x < left+t.widths[c] {
			return c
		}
	}
	return -1
}

// HoldsCell reports whether surface index i is one of the grid's cells.
func (t *Layout) HoldsCell(i int) bool {
	return t != nil && i < len(t.cells) && t.cells[i].Row >= 0
}

// Cell returns the cell coordinate for surface index i.
func (t *Layout) Cell(i int) doc.CellCoord {
	if t == nil || i < 0 || i >= len(t.cells) {
		return doc.CellCoord{Row: -1}
	}
	return t.cells[i]
}

// CellRect returns the bounding rect of cell cc in unit coordinates.
func (t *Layout) CellRect(cc doc.CellCoord) ui.Rect { return t.cell(cc) }
func (t *Layout) Widths() []float32                 { return t.widths }
func (t *Layout) Heights() []float32                { return t.heights }
func (t *Layout) Rows() []float32                   { return t.rows }
func (t *Layout) Columns() []float32                { return t.columns }
func (t *Layout) Natural() []float32                { return t.natural }
func (t *Layout) Overflow() float32                 { return t.overflow }
func (t *Layout) Scroll() float32                   { return t.scroll }
func (t *Layout) ColumnGap() ui.Rect                { return t.columnGap }
func (t *Layout) Parts(box ui.Rect) []ui.Rect       { return t.parts(box) }
func (t *Layout) In() In                            { return t.in }
func (t *Layout) Furthest() (int, int, bool)        { return t.furthest() }
func (t *Layout) GoneCount(axis int) int            { return len(t.gone[axis]) }
func (t *Layout) GoneRoom(axis, i int) (ui.Rect, float32) {
	return t.gone[axis][i].room, t.gone[axis][i].alpha
}

func (in In) Below() float32                             { return in.below }
func (in In) Row() int                                   { return in.row }
func (in In) RowRoom() float32                           { return in.rowRoom }
func (in In) Column() int                                { return in.column }
func (in In) ColumnRoom() float32                        { return in.columnRoom }
func (in In) ApartAt(rows bool, at int) float32          { return in.apartAt(rows, at) }
func (in In) Lines() *lineMotion                         { return in.lines }
func (lm *lineMotion) ExtentOf(a, i int) (float32, bool) { return lm.extentOf(a, i) }
func (lm *lineMotion) OffsetOf(a, i int) float32         { return lm.offsetOf(a, i) }

package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// chromePage is a table page shown in a tester, as the window shows it.
type chromePage struct {
	t     *testing.T
	el    *Element
	table func() *unitLayout
	tt    *ui.Tester
}

func newChromePage(t *testing.T, rows, columns int, cells ...string) *chromePage {
	t.Helper()
	el, table := tablePage(t, rows, columns, cells...)
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	return &chromePage{t: t, el: el, table: table, tt: tt}
}

// at is a page point in the window.
func (c *chromePage) at(x, y float32) (float32, float32) {
	return x + c.el.column, y + c.el.Top - c.el.offset()
}

// move rests the pointer at a page point a moment.
func (c *chromePage) move(x, y float32) {
	wx, wy := c.at(x, y)
	c.tt.Move(wx, wy)
	c.tt.Advance(100 * time.Millisecond)
}

func (c *chromePage) press(x, y float32) {
	wx, wy := c.at(x, y)
	c.tt.Press(wx, wy)
	c.tt.Advance(20 * time.Millisecond)
}

func (c *chromePage) release(x, y float32) {
	wx, wy := c.at(x, y)
	c.tt.Release(wx, wy)
	c.tt.Advance(500 * time.Millisecond)
}

func (c *chromePage) drag(x0, y0, x1, y1 float32) {
	c.press(x0, y0)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		c.move(x0+(x1-x0)*f, y0+(y1-y0)*f)
	}
	c.release(x1, y1)
}

// center is a rectangle's middle.
func center(r ui.Rect) (float32, float32) { return r.X + r.W/2, r.Y + r.H/2 }

// cellPoint is the middle of a cell, in the page.
func (c *chromePage) cellPoint(cc doc.CellCoord) (float32, float32) {
	u := c.table()
	r := u.table.cell(cc)
	return u.x + r.X + r.W/2, u.y + r.Y + r.H/2
}

func (c *chromePage) grid() vocab.Grid {
	c.t.Helper()
	g, ok := vocab.GridOf(c.el.Editor.Engine.Snapshot(), c.table().pres.Unit)
	if !ok {
		c.t.Fatal("no grid")
	}
	return g
}

func (c *chromePage) texts() [][]string {
	g := c.grid()
	rows, columns := g.Size()
	out := make([][]string, rows)
	for r := range rows {
		for col := range columns {
			out[r] = append(out[r], g.Cell(doc.CellCoord{Row: r, Column: col}).Text())
		}
	}
	return out
}

// A click on a row's grip picks its cells whole, and with Shift on the
// next one stretches the pick.
func TestARowsGripPicksItsCells(t *testing.T) {
	c := newChromePage(t, 3, 2, "a", "b", "c", "d", "e", "f")
	c.move(c.cellPoint(doc.CellCoord{Row: 1}))
	if c.el.chrome.near != c.table().pres.Unit || c.el.chrome.row != 1 {
		t.Fatalf("the pointer over row 1 is by %v row %d", c.el.chrome.near, c.el.chrome.row)
	}
	rr, _, _ := c.el.gripRects(c.table(), 1, -1)
	c.move(center(rr))
	c.press(center(rr))
	c.release(center(rr))
	s := c.el.Editor.Selection()
	if s.Kind != doc.SelectCells || !s.Cells.WholeRows || s.Cells.Anchor.Row != 1 || s.Cells.Head.Row != 1 {
		t.Fatalf("a row's grip picked %+v", s)
	}
}

// Dragging a row's grip carries the row among the others, which make room
// for it as it goes, and letting go moves it there in one edit.
func TestARowsGripCarriesIt(t *testing.T) {
	c := newChromePage(t, 3, 2, "a", "b", "c", "d", "e", "f")
	c.move(c.cellPoint(doc.CellCoord{Row: 0}))
	rr, _, _ := c.el.gripRects(c.table(), 0, -1)
	x, y := center(rr)
	_, below := c.cellPoint(doc.CellCoord{Row: 2})
	c.press(x, y)
	c.move(x, y+5)
	c.move(x, below+4)
	u := c.table()
	if c.el.chrome.drag == nil || c.el.chrome.drag.to != 2 {
		t.Fatalf("carried down past the last row, it lands at %+v", c.el.chrome.drag)
	}
	if u.table.lifted != 0 || u.table.dy[1] >= 0 || u.table.dy[2] >= 0 {
		t.Fatalf("the rows below do not make room: lifted %d, offsets %v", u.table.lifted, u.table.dy)
	}
	c.release(x, below+4)
	if got := c.texts(); got[0][0] != "c" || got[1][0] != "e" || got[2][0] != "a" {
		t.Fatalf("the carried row did not move: %v", got)
	}
	if c.table().table.dy[0] != 0 {
		t.Fatal("a moved row is still drawn carried")
	}
	c.el.Editor.Do(vocab.Undo{}, time.Unix(9000, 0))
	if got := c.texts(); got[0][0] != "a" {
		t.Fatalf("one undo does not put the row back: %v", got)
	}
}

// The corner's drag grows the table by whole rows and columns as it lets
// go; Escape drops it.
func TestTheCornerResizesTheTable(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	c.move(c.cellPoint(doc.CellCoord{Row: 1, Column: 1}))
	_, _, corner := c.el.gripRects(c.table(), -1, -1)
	x, y := center(corner)
	u := c.table()
	w, h := u.table.widths[1], u.table.heights[1]
	c.press(x, y)
	c.move(x+w, y+h)
	wx, wy := c.at(x+w, y+h)
	c.tt.Key(0, ui.KeyEscape)
	c.tt.Release(wx, wy)
	c.tt.Advance(time.Second)
	if r, cols := c.grid().Size(); r != 2 || cols != 2 {
		t.Fatalf("an escaped resize made the table %d × %d", r, cols)
	}
	c.move(c.cellPoint(doc.CellCoord{Row: 1, Column: 1}))
	c.drag(x, y, x+w, y+h)
	if r, cols := c.grid().Size(); r != 3 || cols != 3 {
		t.Fatalf("a resize by a row and a column made the table %d × %d", r, cols)
	}
}

// A column's border drags its width, and a double click on it fits the
// column to its text.
func TestAColumnsBorderSetsItsWidth(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	u := c.table()
	r := u.table.cell(doc.CellCoord{Column: 0})
	x, y := u.x+r.X+r.W, u.y+r.Y+r.H/2
	c.move(x, y)
	if cursor, ok := c.el.chromeCursor(x, y); !ok || cursor != ui.CursorResizeEW {
		t.Fatal("a column's border does not show it resizes")
	}
	c.drag(x, y, x-60, y)
	shape := profile.TableShapeOf(c.table().pres.Node)
	if got, want := float32(shape.Columns[0].Width), r.W-60; got < want-1 || got > want+1 {
		t.Fatalf("the border's drag set width %v, want %v", got, want)
	}
	if w := c.table().table.widths[0]; w < r.W-61 || w > r.W-59 {
		t.Fatalf("the column is laid %v wide", w)
	}
	wx, wy := c.at(x-60, y)
	c.tt.ClickAt(wx, wy)
	c.tt.Advance(50 * time.Millisecond)
	c.tt.ClickAt(wx, wy)
	c.tt.Advance(time.Second)
	if shape := profile.TableShapeOf(c.table().pres.Node); !shape.Columns[0].Fit || shape.Columns[0].Width != 0 {
		t.Fatalf("a double click on the border shaped the column %+v", shape.Columns[0])
	}
}

// Resting by the table's foot opens an area there, the page making room
// for it; a click on it makes a row where it stood.
func TestAnAreaByTheFootMakesARow(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	u := c.table()
	b := tableBox(u)
	h0 := u.h
	c.move(b.X+b.W/2, b.Y+b.H+6)
	c.tt.Advance(time.Second)
	u = c.table()
	a, ok := u.table.areaOf()
	if !ok || u.table.in.area != areaRow || u.table.in.at != 2 {
		t.Fatalf("resting by the foot opened %+v", u.table.in)
	}
	if u.h <= h0 || u.shown != u.h {
		t.Fatalf("the area's room is not open: %v from %v, shown %v", u.h, h0, u.shown)
	}
	x, y := u.x+a.X+a.W/2, u.y+a.Y+a.H/2
	c.move(x, y)
	c.press(x, y)
	wx, wy := c.at(x, y)
	c.tt.Release(wx, wy)
	c.tt.Advance(50 * time.Millisecond)
	if r, _ := c.grid().Size(); r != 3 {
		t.Fatalf("a click on the area made %d rows", r)
	}
	if m := c.el.chrome.made; m == nil || m.index != 2 || m.column {
		t.Fatalf("the made row is %+v", m)
	}
}

// In the gutter by the border between two rows, an area opens between
// them, the rows parting around it; one above a border between columns
// opens between them.
func TestAreasOpenBetweenLines(t *testing.T) {
	c := newChromePage(t, 3, 3, "a", "b", "c", "d", "e", "f", "g", "h", "i")
	u := c.table()
	b := tableBox(u)
	top := u.y + u.table.rows[1]
	c.move(b.X-10, top)
	c.tt.Advance(time.Second)
	u = c.table()
	if u.table.in.area != areaRow || u.table.in.at != 1 {
		t.Fatalf("by a border between rows the area is %+v", u.table.in)
	}
	if parts := u.table.parts(u.box); len(parts) != 2 {
		t.Fatalf("the grid is drawn in %d parts around the area", len(parts))
	}
	if u.table.rows[1] <= top-u.y+1 {
		t.Fatal("the rows below the area do not part")
	}
	left := u.x + u.table.cell(doc.CellCoord{Column: 1}).X
	c.move(b.X+b.W/2, b.Y+b.H+200)
	c.tt.Advance(time.Second)
	c.move(left, b.Y-10)
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.in.area != areaColumn || u.table.in.at != 1 {
		t.Fatalf("above a border between columns the area is %+v", u.table.in)
	}
}

// The grips come in as the pointer comes by the table and go as it
// leaves.
func TestGripsComeAndGo(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	id := c.table().pres.Unit
	c.move(c.cellPoint(doc.CellCoord{}))
	c.tt.Advance(time.Second)
	if s := c.el.chrome.grips[id]; s == nil || s.Value() < 0.99 {
		t.Fatal("the grips did not come in")
	}
	b := tableBox(c.table())
	c.move(b.X+b.W/2, b.Y+b.H+300)
	c.tt.Advance(time.Second)
	if s := c.el.chrome.grips[id]; s.Value() > 0.01 {
		t.Fatalf("the grips stay at %v", s.Value())
	}
}

// A carried line lands past each line its leading edge has passed the
// middle of, either way, as far as either end.
func TestACarriedLineLandsByItsLeadingEdge(t *testing.T) {
	starts, sizes := []float32{0, 40, 80}, []float32{40, 40, 40}
	for _, tc := range []struct {
		from int
		top  float32
		want int
	}{
		{0, 0, 0}, {0, 19, 0}, {0, 21, 1}, {0, 80, 2},
		{2, 80, 2}, {2, 61, 2}, {2, 59, 1}, {2, 0, 0},
		{1, 40, 1}, {1, 19, 0}, {1, 61, 2},
	} {
		if got := carryTo(starts, sizes, tc.from, tc.top); got != tc.want {
			t.Errorf("row %d carried to %v lands at %d, want %d", tc.from, tc.top, got, tc.want)
		}
	}
}

// An area's room springs open, the page below gliding with it.
func TestAnAreasRoomSpringsOpen(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	b := tableBox(c.table())
	wx, wy := c.at(b.X+b.W/2, b.Y+b.H+6)
	c.tt.Move(wx, wy)
	var opening bool
	for range 90 {
		c.tt.Advance(16 * time.Millisecond)
		if u := c.table(); u.table.in.area != areaNone && u.shown < u.h-0.5 {
			opening = true
		}
	}
	if !opening {
		t.Fatal("the area's room did not spring open")
	}
	if u := c.table(); u.shown != u.h {
		t.Fatalf("the room settles at %v of %v", u.shown, u.h)
	}
}

// A secondary click on a cell puts the caret there and opens the table's
// commands, as they would act, then the text's editing items.
func TestACellsMenuHasTheTablesCommands(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	wx, wy := c.at(c.cellPoint(doc.CellCoord{Row: 0, Column: 1}))
	c.tt.RightClickAt(wx, wy)
	c.tt.Advance(100 * time.Millisecond)
	if at := cellOf(t, c.el, c.el.Editor.Selection().Head); at != (doc.CellCoord{Row: 0, Column: 1}) {
		t.Fatalf("a secondary click put the caret in %v", at)
	}
	items := c.tt.Menu()
	for _, want := range []string{"Insert row above", "Delete column", "Delete table"} {
		found := false
		for _, it := range items {
			found = found || it == want
		}
		if !found {
			t.Fatalf("the menu %q has no %q", items, want)
		}
	}
	if err := c.tt.ChooseMenuItem("Insert row above"); err != nil {
		t.Fatal(err)
	}
	c.tt.Advance(100 * time.Millisecond)
	if got := c.texts(); len(got) != 3 || got[1][0] != "a" {
		t.Fatalf("the menu's insert made %v", got)
	}
}

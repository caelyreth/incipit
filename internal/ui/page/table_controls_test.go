package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/page/table"
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
	r := u.table.CellRect(cc)
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
	if row, _ := table.GripLines(c.el.tableHost(), c.el.tableUnit(c.table())); c.el.near != c.table().pres.Unit || row != 1 {
		t.Fatalf("the pointer over row 1 is by %v row %d", c.el.near, row)
	}
	rr, _, _ := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), 1, -1)
	c.move(center(rr))
	c.press(center(rr))
	c.release(center(rr))
	s := c.el.Editor.Selection()
	if s.Kind != doc.SelectCells || !s.Cells.WholeRows || s.Cells.Anchor.Row != 1 || s.Cells.Head.Row != 1 {
		t.Fatalf("a row's grip picked %+v", s)
	}
}

// Dragging a row's grip carries a copy of the row with the pointer, the
// row dimmed in its place; once the pointer has stayed by a place the grid
// parts there for it, and letting go moves it there in one edit, the copy
// landing as the room it left closes.
func TestARowsGripCarriesIt(t *testing.T) {
	c := newChromePage(t, 3, 2, "a", "b", "c", "d", "e", "f")
	c.move(c.cellPoint(doc.CellCoord{Row: 0}))
	rr, _, _ := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), 0, -1)
	x, y := center(rr)
	_, below := c.cellPoint(doc.CellCoord{Row: 2})
	h0 := c.table().h
	c.press(x, y)
	c.move(x, y+5)
	c.move(x, below+4)
	c.tt.Advance(time.Second)
	u := c.table()
	if to, ok := table.LineCarryTo(c.el.tableHost()); !ok || to != 3 {
		t.Fatalf("carried down past the last row's middle, it lands at %+v", c.el.drag)
	}
	if a := u.table.In().ApartAt(true, 3); a < u.table.Heights()[0]-1 || u.h < h0+a-1 {
		t.Fatalf("the grid does not part past the last row: room %v, height %v from %v", a, u.h, h0)
	}
	c.release(x, below+4)
	if got := c.texts(); got[0][0] != "c" || got[1][0] != "e" || got[2][0] != "a" {
		t.Fatalf("the carried row did not move: %v", got)
	}
	if u := c.table(); u.table.In() != (table.In{}) || u.h != h0 {
		t.Fatalf("a moved row leaves room %+v, height %v from %v", u.table.In(), u.h, h0)
	}
	c.el.Editor.Do(vocab.Undo{}, time.Unix(9000, 0))
	if got := c.texts(); got[0][0] != "a" {
		t.Fatalf("one undo does not put the row back: %v", got)
	}
	// Undone, the rows glide back to their places, the one carried
	// furthest over the rest, and the table keeps its height.
	c.tt.Frame()
	c.tt.Advance(32 * time.Millisecond)
	u = c.table()
	if axis, index, ok := u.table.Furthest(); !ok || axis != 0 || index != 0 || u.h != h0 {
		t.Fatalf("undone, row %d of axis %d glides furthest (%v), height %v from %v", index, axis, ok, u.h, h0)
	}
	if o := u.table.In().Lines().OffsetOf(0, 0); o <= 0 {
		t.Fatalf("undone, the carried row glides from %v, want from below", o)
	}
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.In() != (table.In{}) {
		t.Fatalf("undone, the rows settle as %+v", u.table.In())
	}
}

// Let go by its own place, a carried row stays, and so does the grid.
func TestARowLetGoInItsPlaceStays(t *testing.T) {
	c := newChromePage(t, 3, 2, "a", "b", "c", "d", "e", "f")
	c.move(c.cellPoint(doc.CellCoord{Row: 1}))
	rr, _, _ := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), 1, -1)
	x, y := center(rr)
	c.press(x, y)
	c.move(x, y+6)
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.In() != (table.In{}) {
		t.Fatalf("a row carried by its place parts the grid: %+v", u.table.In())
	}
	c.release(x, y+6)
	if got := c.texts(); got[1][0] != "c" {
		t.Fatalf("a row let go in its place moved: %v", got)
	}
}

// The corner's drag grows the table by whole rows and columns as it lets
// go; Escape drops it.
func TestTheCornerResizesTheTable(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	c.move(c.cellPoint(doc.CellCoord{Row: 1, Column: 1}))
	_, _, corner := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), -1, -1)
	x, y := center(corner)
	u := c.table()
	w, h := u.table.Widths()[1], u.table.Heights()[1]
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
	r := u.table.CellRect(doc.CellCoord{Column: 0})
	x, y := u.x+r.X+r.W, u.y+r.Y+r.H/2
	c.move(x, y)
	if cursor, ok := c.el.cursorAt(x, y); !ok || cursor != ui.CursorResizeEW {
		t.Fatal("a column's border does not show it resizes")
	}
	c.drag(x, y, x-60, y)
	shape := profile.TableShapeOf(c.table().pres.Node)
	if got, want := float32(shape.Columns[0].Width), r.W-60; got < want-1 || got > want+1 {
		t.Fatalf("the border's drag set width %v, want %v", got, want)
	}
	if w := c.table().table.Widths()[0]; w < r.W-61 || w > r.W-59 {
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
	b := table.Box(c.el.tableUnit(u))
	h0 := u.h
	c.move(b.X+b.W/2, b.Y+b.H+6)
	c.tt.Advance(time.Second)
	u = c.table()
	_, _, step, _ := table.AreaRooms(c.el.tableHost())
	if u.table.In().Below() != step || u.h != h0+step {
		t.Fatalf("resting by the foot opened %+v, height %v from %v", u.table.In(), u.h, h0)
	}
	areas := table.Areas(c.el.tableHost(), c.el.tableUnit(u))
	if len(areas) != 1 || areas[0].Part != table.PartBelow || areas[0].Alpha != 1 {
		t.Fatalf("the foot shows %+v", areas)
	}
	x, y := center(areas[0].Box)
	c.move(x, y)
	c.press(x, y)
	wx, wy := c.at(x, y)
	c.tt.Release(wx, wy)
	c.tt.Advance(50 * time.Millisecond)
	if r, _ := c.grid().Size(); r != 3 {
		t.Fatalf("a click on the area made %d rows", r)
	}
	if idx, col, from, ok := c.el.tableHost().State(c.table().pres.Unit, false).MadeInfo(); !ok || idx != 2 || col || from <= 0 {
		t.Fatalf("the made row is %d %v %v %v", idx, col, from, ok)
	}
	// It comes in from the area's room, the table's height going on from
	// where the area left it.
	u = c.table()
	if e, ok := u.table.In().Lines().ExtentOf(0, 2); !ok || e >= u.table.Natural()[2] || e < 0.5*min(step, u.table.Natural()[2]) {
		t.Fatalf("the made row comes in at %v of %v", e, u.table.Natural()[2])
	}
	if u.h > h0+step+u.table.Heights()[2] {
		t.Fatalf("the made row jumps the table to %v", u.h)
	}
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.In().Lines() != nil {
		t.Fatalf("the made row is still coming in: %+v", u.table.In().Lines())
	}
}

// In the gutter by the border between two rows, an area opens between
// them, the rows parting around it; one above a border between columns
// opens between them.
func TestAreasOpenBetweenLines(t *testing.T) {
	c := newChromePage(t, 3, 3, "a", "b", "c", "d", "e", "f", "g", "h", "i")
	u := c.table()
	b := table.Box(c.el.tableUnit(u))
	top := u.y + u.table.Rows()[1]
	c.move(b.X-10, top)
	c.tt.Advance(time.Second)
	u = c.table()
	_, _, _, between := table.AreaRooms(c.el.tableHost())
	if u.table.In().Row() != 1 || u.table.In().RowRoom() != between {
		t.Fatalf("by a border between rows the area is %+v", u.table.In())
	}
	if abs32(c.el.scroll-between/2) > 0.01 {
		t.Fatalf("the page lifts %v, want half the room, %v", c.el.scroll, between/2)
	}
	if parts := u.table.Parts(u.box); len(parts) != 2 {
		t.Fatalf("the grid is drawn in %d parts around the area", len(parts))
	}
	if u.table.Rows()[1] <= top-u.y+1 {
		t.Fatal("the rows below the area do not part")
	}
	left := u.x + u.table.CellRect(doc.CellCoord{Column: 1}).X
	c.move(b.X+b.W/2, b.Y+b.H+200)
	c.tt.Advance(time.Second)
	c.move(left, b.Y-10)
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.In().Column() != 1 || u.table.In().ColumnRoom() != between || u.table.ColumnGap().W != between {
		t.Fatalf("above a border between columns the area is %+v", u.table.In())
	}
}

// The grips come in as the pointer comes by the table and go as it
// leaves.
func TestGripsComeAndGo(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	id := c.table().pres.Unit
	c.move(c.cellPoint(doc.CellCoord{}))
	c.tt.Advance(time.Second)
	if v := c.el.springs.value(springKey{unit: id, name: "grips", of: shareValue}); v < 0.99 {
		t.Fatal("the grips did not come in")
	}
	b := table.Box(c.el.tableUnit(c.table()))
	c.move(b.X+b.W/2, b.Y+b.H+300)
	c.tt.Advance(time.Second)
	if v := c.el.springs.value(springKey{unit: id, name: "grips", of: shareValue}); v > 0.01 {
		t.Fatalf("the grips stay at %v", v)
	}
}

// A row the table gains comes in from nothing, the page after it gliding
// with it; one it loses, as that is undone, closes, drawn as it was, the
// page after it gliding up with it.
func TestANewRowComesIn(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	h0 := c.table().h
	c.el.Editor.Do(vocab.Command{ID: "table.row-below"}, time.Unix(7000, 0))
	c.tt.Frame()
	c.tt.Advance(16 * time.Millisecond)
	u := c.table()
	if e, ok := u.table.In().Lines().ExtentOf(0, 2); !ok || e >= 0.5*u.table.Natural()[2] {
		t.Fatalf("the new row comes in at %v of %v", e, u.table.Natural()[2])
	}
	if u.h >= h0+u.table.Heights()[1] {
		t.Fatalf("the new row jumps the table to %v from %v", u.h, h0)
	}
	c.tt.Advance(time.Second)
	u = c.table()
	if u.table.In() != (table.In{}) || u.shown != u.h {
		t.Fatalf("the new row settles as %+v, shown %v of %v", u.table.In(), u.shown, u.h)
	}
	h1 := u.h
	c.el.Editor.Do(vocab.Undo{}, time.Unix(7001, 0))
	c.tt.Frame()
	c.tt.Advance(80 * time.Millisecond)
	u = c.table()
	if u.table.GoneCount(0) != 1 || u.h <= h0 || u.h >= h1 {
		t.Fatalf("the lost row goes (%d), the table at %v between %v and %v", u.table.GoneCount(0), u.h, h0, h1)
	}
	c.tt.Advance(time.Second)
	if u := c.table(); u.table.GoneCount(0) != 0 || u.h != h0 {
		t.Fatalf("the lost row settles (%d), the table at %v of %v", u.table.GoneCount(0), u.h, h0)
	}
}

// An area's room eases open, the page below gliding with it, and the area
// comes in as it does.
func TestAnAreasRoomEasesOpen(t *testing.T) {
	c := newChromePage(t, 2, 2, "a", "b", "c", "d")
	b := table.Box(c.el.tableUnit(c.table()))
	_, _, step, _ := table.AreaRooms(c.el.tableHost())
	wx, wy := c.at(b.X+b.W/2, b.Y+b.H+6)
	c.tt.Move(wx, wy)
	var opening bool
	last := float32(0)
	for range 90 {
		c.tt.Advance(16 * time.Millisecond)
		room := c.table().table.In().Below()
		if room < last {
			t.Fatalf("the room closes on its way open: %v after %v", room, last)
		}
		if room > 0 && room < step-1 {
			opening = true
			if a := table.Areas(c.el.tableHost(), c.el.tableUnit(c.table())); len(a) == 1 && a[0].Alpha >= 1 {
				t.Fatalf("the area shows whole in a room %v of %v", room, step)
			}
		}
		last = room
	}
	if !opening || last != step {
		t.Fatalf("the area's room did not ease open: it stands at %v of %v", last, step)
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

// A secondary click on a column's grip picks the column and offers what
// acts on a column alone; on a row's, a row's.
func TestALinesGripMenuIsItsLines(t *testing.T) {
	c := newChromePage(t, 2, 3, "a", "b", "c", "d", "e", "f")
	c.move(c.cellPoint(doc.CellCoord{Row: 1, Column: 1}))
	_, cr, _ := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), -1, 1)
	c.move(center(cr))
	wx, wy := c.at(center(cr))
	c.tt.RightClickAt(wx, wy)
	c.tt.Advance(100 * time.Millisecond)
	if s := c.el.Editor.Selection(); s.Kind != doc.SelectCells || !s.Cells.WholeColumns || s.Cells.Head.Column != 1 {
		t.Fatalf("a secondary click on a column's grip picked %+v", s)
	}
	items := c.tt.Menu()
	has := func(want string) bool {
		for _, it := range items {
			if it == want {
				return true
			}
		}
		return false
	}
	if !has("Delete column") || !has("Insert column left") || has("Insert row above") || has("Delete row") || has("Delete table") {
		t.Fatalf("a column's grip offers %q", items)
	}
	if err := c.tt.ChooseMenuItem("Delete column"); err != nil {
		t.Fatal(err)
	}
	c.tt.Advance(100 * time.Millisecond)
	if got := c.texts(); len(got[0]) != 2 || got[0][1] != "c" {
		t.Fatalf("the column's delete left %v", got)
	}
	c.move(c.cellPoint(doc.CellCoord{Row: 1, Column: 0}))
	rr, _, _ := table.GripRects(c.el.tableHost(), c.el.tableUnit(c.table()), 1, -1)
	c.move(center(rr))
	wx, wy = c.at(center(rr))
	c.tt.RightClickAt(wx, wy)
	c.tt.Advance(100 * time.Millisecond)
	items = c.tt.Menu()
	if !has("Delete row") || has("Delete column") || has("Insert column left") {
		t.Fatalf("a row's grip offers %q", items)
	}
}

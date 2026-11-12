package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// tablePage is a page with a paragraph over a table of rows by columns,
// its cells typed in order with Tab, and the caret in the last.
func tablePage(t *testing.T, rows, columns int, cells ...string) (*Element, func() *unitLayout) {
	t.Helper()
	el := newElement(t)
	typeText(el, "above")
	clock := int64(1000)
	do := func(in vocab.Intent) {
		clock++
		el.Editor.Do(in, time.Unix(clock, 0))
	}
	do(vocab.Break{})
	do(profile.InsertTable{Rows: rows, Columns: columns})
	for i, c := range cells {
		if i > 0 {
			do(vocab.Leave{})
		}
		for _, r := range c {
			do(vocab.Type{Text: string(r)})
		}
	}
	table := func() *unitLayout {
		for _, u := range el.relayout().units {
			if u.table != nil {
				return u
			}
		}
		t.Fatal("no table")
		return nil
	}
	return el, table
}

// cellOf is the cell a caret stands in, as a coordinate.
func cellOf(t *testing.T, el *Element, c doc.Caret) doc.CellCoord {
	t.Helper()
	_, at, ok := vocab.Locate(el.Editor.Engine.Snapshot(), c.Pos.Node)
	if !ok {
		t.Fatalf("%v is in no cell", c)
	}
	return at
}

// moveTo moves the caret to a cell's start, as a click there would.
func moveTo(t *testing.T, el *Element, u *unitLayout, cc doc.CellCoord) {
	t.Helper()
	for i, s := range u.surfaces {
		if u.table.cells[i] == cc {
			el.Editor.Do(vocab.Place{Stop: s.stops[0].caret}, time.Unix(5000, 0))
			el.relayout()
			return
		}
	}
	t.Fatalf("no cell %v", cc)
}

func vertically(t *testing.T, el *Element, up bool) doc.Caret {
	t.Helper()
	dir := vocab.Down
	if up {
		dir = vocab.Up
	}
	in, ok := el.Editor.resolve(vocab.VisualMove{Dir: dir}, 600)
	if !ok {
		t.Fatal("no move")
	}
	return in.Target
}

// Up and Down move between rows in the column at the goal, and a cell's
// own lines first: a short cell beside a tall one goes to the row below,
// not to its neighbour's next line.
func TestVerticalMotionWalksTheGridsRows(t *testing.T) {
	long := strings.Repeat("wrapping words ", 30)
	el, table := tablePage(t, 3, 2, "a", "b", "c", long, "e", "f")
	u := table()
	moveTo(t, el, u, doc.CellCoord{Row: 1, Column: 0})
	if at := cellOf(t, el, vertically(t, el, false)); at != (doc.CellCoord{Row: 2, Column: 0}) {
		t.Fatalf("down from a short cell beside a tall one went to %v", at)
	}
	if at := cellOf(t, el, vertically(t, el, true)); at != (doc.CellCoord{Row: 0, Column: 0}) {
		t.Fatalf("up went to %v", at)
	}
	moveTo(t, el, u, doc.CellCoord{Row: 1, Column: 1})
	c := vertically(t, el, false)
	if at := cellOf(t, el, c); at != (doc.CellCoord{Row: 1, Column: 1}) {
		t.Fatalf("down in a wrapped cell left it for %v", at)
	}
}

// A point on a cell's room, its text or the padding around it, is in
// that cell; past what a scrolled table shows, a point is read at its
// edge.
func TestClicksInACellsRoomLandInIt(t *testing.T) {
	el, table := tablePage(t, 2, 2, "a", "b", "c", "d")
	u := table()
	t1 := u.table
	cell := t1.cell(doc.CellCoord{Row: 1, Column: 1})
	// The cell's corner, in its padding.
	c, _, ok := el.Editor.layout.hit(u.x+cell.X+2, u.y+cell.Y+2)
	if !ok || cellOf(t, el, c) != (doc.CellCoord{Row: 1, Column: 1}) {
		t.Fatalf("a click in a cell's padding hit %v", c)
	}
	// Past the table's end, level with its last row.
	c, _, ok = el.Editor.layout.hit(u.x+u.box.X+u.box.W+200, u.y+cell.Y+cell.H/2)
	if !ok || cellOf(t, el, c) != (doc.CellCoord{Row: 1, Column: 1}) {
		t.Fatalf("a click past the table hit %v", c)
	}
}

// Columns whose text fits on a line take the text's width between them;
// a table asking more wraps, spreads into the margins, and past them
// scrolls across within what it shows.
func TestColumnsTakeTheirRoom(t *testing.T) {
	m := newElement(t).Editor.metrics
	el, table := tablePage(t, 1, 2, "a", "b")
	u := table()
	if w := u.table.widths[0] + u.table.widths[1]; w != float32(int(m.Width+0.5)) && w != m.Width {
		t.Fatalf("a short table is %v wide, want the column's %v", w, m.Width)
	}
	if u.table.overflow != 0 || u.box.X != 0 {
		t.Fatalf("a short table scrolls %v from %v", u.table.overflow, u.box.X)
	}
	_ = el
	var cells []string
	for range 10 {
		cells = append(cells, "Superlongwordthatneverbreaks")
	}
	el, table = tablePage(t, 1, 10, cells...)
	u = table()
	m = el.Editor.metrics
	if u.table.overflow <= 0 {
		t.Fatal("a table wider than the margins does not scroll")
	}
	if u.box.X != -m.Reach || u.box.W != m.Width+2*m.Reach {
		t.Fatalf("a scrolling table shows %v, want the margins' reach %v", u.box, m.Reach)
	}
	for i, w := range u.table.widths {
		if s := u.surfaces[i]; len(s.layout.Lines()) != 1 {
			t.Fatalf("column %d (%v wide) breaks its word", i, w)
		}
	}
}

// The wheel turned sideways over a table scrolls it, as far as it
// overflows, and moving into a cell it hides glides it into view.
func TestAWideTableScrollsAcross(t *testing.T) {
	var cells []string
	for range 10 {
		cells = append(cells, "Superlongwordthatneverbreaks")
	}
	el, table := tablePage(t, 1, 10, cells...)
	u := table()
	// Typing in the last cell revealed it.
	el.follow(el.relayout(), time.Unix(6000, 0))
	el.scrollTables(el.Editor.layout, time.Unix(7000, 0), true, false)
	if s := table().table.scroll; s <= 0 {
		t.Fatalf("the last cell is not revealed: scroll %v", s)
	}
	moveTo(t, el, u, doc.CellCoord{Column: 0})
	el.Editor.reveal = true
	el.follow(el.relayout(), time.Unix(8000, 0))
	el.scrollTables(el.Editor.layout, time.Unix(9000, 0), true, false)
	if s := table().table.scroll; s != 0 {
		t.Fatalf("the first cell is not revealed: scroll %v", s)
	}
	u = table()
	el.pointer = ui.Point{X: el.column + u.box.X + 20, Y: el.Top + u.y + 10}
	el.handle(ui.InputEvent{Kind: ui.InputScroll, DX: 150, Precise: true, Time: time.Unix(9100, 0)}, ui.Services{})
	if s := table().table.scroll; s != 150 {
		t.Fatalf("a sideways scroll moved the table to %v", s)
	}
	if el.ScrollY() != 0 {
		t.Fatalf("a sideways scroll moved the page to %v", el.ScrollY())
	}
}

// A cell range fills its cells, an empty one as much as one with text.
func TestACellRangeFillsItsCells(t *testing.T) {
	el, table := tablePage(t, 2, 2, "a", "b", "c", "")
	u := table()
	el.Editor.Do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true}, time.Unix(6000, 0))
	pl := el.relayout()
	if s := el.Editor.Selection(); s.Kind != doc.SelectCells {
		t.Fatalf("selection %+v, want a cell range", s)
	}
	u = table()
	var boxes []ui.Rect
	for _, pt := range el.Editor.deco.Paints {
		if pt.Group != vocab.GroupSelection {
			continue
		}
		for _, c := range pt.Covers {
			for _, pc := range pl.pieces(u, c) {
				for _, r := range pc.rows {
					boxes = append(boxes, r.box)
				}
			}
		}
	}
	if len(boxes) != 2 {
		t.Fatalf("a range over two cells paints %v", boxes)
	}
	for i, cc := range []doc.CellCoord{{Row: 1, Column: 0}, {Row: 1, Column: 1}} {
		want := outset(u.table.cell(cc), -decoPad, -decoPad/2)
		want.X += u.x
		want.Y += u.y
		if boxes[i] != want {
			t.Fatalf("cell %v painted %v, want its box %v", cc, boxes[i], want)
		}
	}
}

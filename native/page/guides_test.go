package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// A selected scope shows Select All's key over the wider scope it would
// select, as a bracket in the margin out past the controls' lane, growing
// out of the rows selected now.
func TestARangeGuideBracketsWhatItWouldSelect(t *testing.T) {
	el := newElement(t)
	typeText(el, "First paragraph.")
	el.Editor.Do(vocab.Break{}, time.Unix(900, 0))
	typeText(el, "Then (a pair")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(el.Theme.Surface.Page).Children(func() { el.View(c) })
	}, 900, 360)
	el.Focus()
	tt.Frame()
	tt.Advance(time.Second)
	if g := el.shownGuides()["core.selectAll"]; g != nil {
		t.Fatalf("a caret shows Select All as %+v", g)
	}
	el.Editor.Do(vocab.SelectScope{}, time.Unix(1000, 0))
	tt.Frame()
	tt.Advance(time.Second)
	el.Editor.Do(vocab.SelectScope{}, time.Unix(1001, 0))
	tt.Frame()
	pl := el.Editor.layout
	from, _, _ := pl.span(selectionCovers(el.Editor.deco.Paints))
	if y := el.springs.value(guideKey("core.selectAll", "y", placeValue)); y != from.Y {
		t.Fatalf("the bracket starts at %v, want the selected rows' top %v", y, from.Y)
	}
	tt.Advance(time.Second)
	g := el.shownGuides()["core.selectAll"]
	if g == nil || g.kind != spotRange || len(g.keys) == 0 {
		t.Fatalf("a selected scope shows Select All as %+v", g)
	}
	first, last := pl.units[0], pl.units[len(pl.units)-1]
	if g.to.Y > first.y+first.h/2 || g.to.Y+g.to.H < last.y+last.h/2 {
		t.Fatalf("the bracket stands over %+v, want both paragraphs", g.to)
	}
	if v := el.springs.value(guideKey("core.selectAll", "shown", shareValue)); v < 0.99 {
		t.Fatalf("the bracket shows %v", v)
	}
}

func TestListIndentAndTableNewRowGuidesRender(t *testing.T) {
	el := newElement(t)
	typeText(el, "- one")
	el.Editor.Do(vocab.Break{}, time.Unix(901, 0))
	typeText(el, "two")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(el.Theme.Surface.Page).Children(func() { el.View(c) })
	}, 900, 360)
	el.Focus()
	tt.Frame()
	tt.Advance(time.Second)

	shown := el.shownGuides()
	nest := shown["lists.nest"]
	outdent := shown["lists.outdent"]
	if nest == nil || !nest.offered || len(nest.keys) == 0 {
		t.Fatalf("lists.nest guide missing or hidden: %+v", nest)
	}
	if outdent == nil || !outdent.offered || len(outdent.keys) == 0 {
		t.Fatalf("lists.outdent guide missing or hidden: %+v", outdent)
	}
	if outdent.to.X >= nest.to.X {
		t.Fatalf("outdent X (%v) should sit left of nest X (%v)", outdent.to.X, nest.to.X)
	}

	elTable, _ := tablePage(t, 2, 2, "a", "b", "c", "d")
	ttTable := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(elTable.Theme.Surface.Page).Children(func() { elTable.View(c) })
	}, 900, 480)
	elTable.Focus()
	ttTable.Frame()
	ttTable.Advance(time.Second)
	leave := elTable.shownGuides()["core.leave"]
	if leave == nil || !leave.offered || leave.kind != spotGapAfter || len(leave.keys) == 0 {
		t.Fatalf("last-cell core.leave guide = %+v", leave)
	}
}

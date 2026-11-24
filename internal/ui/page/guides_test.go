package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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

// Tab and Shift+Tab on an item mark the edge they move, on its first line
// however far it wraps, never in its words: Tab the item's leading edge,
// where its number starts; Shift+Tab the level out in the indent, its keys
// ending there, apart from Tab's.
func TestIndentGuidesStandAtTheItemsEdge(t *testing.T) {
	el := newElement(t)
	typeText(el, "1. one")
	el.Editor.Do(vocab.Break{}, time.Unix(901, 0))
	typeText(el, "The tab key hint may need to cover more place like list indent, and so on and on until it wraps onto more lines than one, and then more.")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(el.Theme.Surface.Page).Children(func() { el.View(c) })
	}, 600, 400)
	el.Focus()
	tt.Frame()
	tt.Advance(time.Second)

	pl := el.Editor.layout
	u := pl.units[len(pl.units)-1]
	if pl.units[0] == u {
		t.Fatal("one unit")
	}
	caret, _ := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	first, _ := pl.caretBox(doc.Caret{Pos: doc.Pos{Node: u.pres.Unit}}, el.Editor.metrics)
	if caret.Y <= first.Y {
		t.Fatal("the item does not wrap")
	}
	shown := el.shownGuides()
	nest, outdent := shown["lists.nest"], shown["lists.outdent"]
	if nest == nil || outdent == nil {
		t.Fatalf("guides nest %v, outdent %v", nest, outdent)
	}
	onFirst := func(y float32) bool { return y > first.Y-first.H/2 && y < first.Y+first.H/2 }
	if nest.to.X != u.x || !onFirst(nest.to.Y) || nest.align != ui.Start {
		t.Fatalf("Tab's mark at %v (%v), want the item's edge %v on its first line %v", nest.to, nest.align, u.x, first.Y)
	}
	if outdent.to.X >= u.x || !onFirst(outdent.to.Y) || outdent.align != ui.End {
		t.Fatalf("Shift+Tab's mark at %v (%v), want out in the indent before %v, keys ending at it", outdent.to, outdent.align, u.x)
	}
}

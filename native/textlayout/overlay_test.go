package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// An overlay shows a flow as a view has it, the flow as the editor says
// it: what a stale formula last set in place of its source, a caption
// line as far as it has opened or, without a say, as far as it has a
// caption, a figure as wide as a resize sets it.
func TestAnOverlayShowsTheFlowAsTheViewHasIt(t *testing.T) {
	f := surface.Formula{Range: model.NewUTF16Range(2, 7), Text: model.NewUTF16Range(3, 6), Source: `x+\`}
	stale := &Overlay{Formulas: map[FormulaKey]FormulaShow{KeyOf(f): {Open: true, Card: 2, Stale: true, Last: "x"}}}
	if shown := stale.formula(f); shown.Set != "x" || shown.Source != `x+\` || shown.Card != 1 || !shown.Open {
		t.Fatalf("stale formula shows %+v", shown)
	}
	if shown := (*Overlay)(nil).formula(f); shown.Set != f.Source || shown.Open || shown.Card != 0 {
		t.Fatalf("no overlay shows %+v", shown)
	}
	figure := surface.Figure{Node: 9, Width: 100, Height: 50, Span: 0.5, Captioned: true}
	none := (*Overlay)(nil)
	if none.caption(figure) != 1 || none.caption(surface.Figure{Node: 9}) != 0 || none.span(figure) != 0.5 {
		t.Fatal("without an overlay a figure is not as its flow says")
	}
	opening := &Overlay{Captions: map[model.NodeID]float32{9: 0.25}, Spans: map[model.NodeID]float32{9: 1}}
	if opening.caption(figure) != 0.25 || opening.span(figure) != 1 {
		t.Fatal("the overlay's caption or share was not taken")
	}
}

// A layout is kept while its flow and an overlay alike are, and laid out
// again as the overlay changes, the overlay it took its own.
func TestALayoutIsKeptForAnEqualOverlay(t *testing.T) {
	f := surface.Formula{Range: model.NewUTF16Range(2, 7), Text: model.NewUTF16Range(3, 6), Source: "x^2"}
	flow := formulaFlow(t, "a $x^2$ b\nafter", f)
	cache := NewCache()
	overlay := opened(0.5, f)
	first := cache.Update(flow, overlay, 400, testStyle())
	if again := cache.Update(flow, opened(0.5, f), 400, testStyle()); again != first {
		t.Fatal("an equal overlay laid the page out again")
	}
	overlay.Formulas[KeyOf(f)] = FormulaShow{Open: true, Card: 1}
	if kept := cache.Update(flow, opened(0.5, f), 400, testStyle()); kept != first {
		t.Fatal("the cache shared the overlay it was given")
	}
	if next := cache.Update(flow, opened(1, f), 400, testStyle()); next == first || next.FormulaCards()[0].Shown != 1 {
		t.Fatal("a changed overlay kept the last layout")
	}
	if !(*Overlay)(nil).Equal(&Overlay{}) || (&Overlay{Rooms: []Room{{At: -1, Height: 1}}}).Equal(nil) {
		t.Fatal("an empty overlay is not the zero overlay")
	}
	tables := func(scroll float32) *Overlay {
		return &Overlay{Tables: map[model.NodeID]TableRoom{1: {Scroll: scroll, RowShown: []float32{1}}}}
	}
	if !tables(1).Equal(tables(1)) || tables(1).Equal(tables(2)) {
		t.Fatal("table rooms compare by what they open")
	}
}

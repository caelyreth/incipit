package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

func formulaFlow(t *testing.T, text string, formulas ...surface.Formula) *surface.Flow {
	t.Helper()
	flow, err := testFlow(t, text, 0).WithFormulas(formulas)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

// A formula the caret is not in is set as one object of its line, on the
// baseline, its notation and source mapped to either side of it; an open
// one is laid out as text.
func TestInlineFormulaIsSetInItsLine(t *testing.T) {
	text := "a $x^2$ b"
	set := surface.Formula{Range: model.NewUTF16Range(2, 7), Text: model.NewUTF16Range(3, 6), Source: "x^2"}
	layout := NewCache().Update(formulaFlow(t, text, set), 400, testStyle())
	placed := layout.Formulas()
	if len(placed) != 1 || placed[0].Box == nil || placed[0].Rect.W <= 0 {
		t.Fatalf("placed = %+v", placed)
	}
	box := placed[0].Box
	if got := placed[0].Origin.Y - placed[0].Rect.Y; got != box.Ascent {
		t.Fatalf("baseline %v below the object's top, ascent %v", got, box.Ascent)
	}
	before, after := layout.Caret(2), layout.Caret(7)
	if after.X-before.X < box.Width-0.5 {
		t.Fatalf("formula from %v to %v is narrower than its box %v", before.X, after.X, box.Width)
	}
	middle := ui.Point{X: placed[0].Rect.X + placed[0].Rect.W/2, Y: placed[0].Rect.Y + placed[0].Rect.H/2}
	if found, ok := layout.FormulaAt(middle); !ok || found.Formula.Range != set.Range {
		t.Fatalf("formula at its middle = %+v, %v", found, ok)
	}

}

// A formula block is one line of its own, its fences and source lines
// joined, while it is edited too.
func TestFormulaBlockIsOneLine(t *testing.T) {
	text := "before\n$$\n\\frac12\n$$\nafter"
	block := surface.Formula{
		Range: model.NewUTF16Range(7, 19), Text: model.NewUTF16Range(10, 16),
		Source: "\\frac12", Display: true,
	}
	layout := NewCache().Update(formulaFlow(t, text, block), 400, testStyle())
	if got := len(layout.Paragraphs()); got != 3 {
		t.Fatalf("paragraphs = %d", got)
	}
	placed := layout.Formulas()
	if len(placed) != 1 || placed[0].Rect.H < placed[0].Box.Ascent+placed[0].Box.Descent-0.5 {
		t.Fatalf("placed = %+v", placed)
	}
	middle := ui.Point{X: placed[0].Rect.X + placed[0].Rect.W/2, Y: placed[0].Rect.Y + placed[0].Rect.H/2}
	if got := layout.Hit(middle); got != block.Range.Start && got != block.Range.End {
		t.Fatalf("hit in the formula = %d", got)
	}

	block.Open, block.Card = true, 1
	open := NewCache().Update(formulaFlow(t, text, block), 400, testStyle())
	if got := len(open.Paragraphs()); got != 3 || len(open.Formulas()) != 1 {
		t.Fatalf("edited block has %d paragraphs, %d set", got, len(open.Formulas()))
	}
}

// An inline formula being edited stays set in its line, its paragraph
// unmoved, and opens its card below the paragraph as far as it is shown:
// the source across the top, where the caret and selection in it are, and
// what it sets centered beneath. Arrows leave the source past its ends.
func TestInlineFormulaEditsOnItsCard(t *testing.T) {
	text := "a $x^2$ b\nafter"
	f := surface.Formula{Range: model.NewUTF16Range(2, 7), Text: model.NewUTF16Range(3, 6), Source: "x^2"}
	closed := NewCache().Update(formulaFlow(t, text, f), 400, testStyle())
	f.Open = true
	half, full := f, f
	half.Card, full.Card = 0.5, 1
	halfway := NewCache().Update(formulaFlow(t, text, half), 400, testStyle())
	layout := NewCache().Update(formulaFlow(t, text, full), 400, testStyle())

	if got, want := layout.Caret(8), closed.Caret(8); got != want {
		t.Fatalf("the line moved as its formula opened: %+v, was %+v", got, want)
	}
	room := layout.Caret(10).Y - closed.Caret(10).Y
	if partial := halfway.Caret(10).Y - closed.Caret(10).Y; room <= 0 || partial < room/2-0.5 || partial > room/2+0.5 {
		t.Fatalf("room below: %v shown, %v halfway", room, partial)
	}
	cards := layout.FormulaCards()
	if len(cards) != 1 || !cards[0].HasSet || cards[0].Shown != 1 {
		t.Fatalf("cards = %+v", cards)
	}
	card := cards[0]
	line := layout.Caret(8)
	if card.Rect.X != 0 || card.Rect.W != 400 || card.Rect.Y < line.Y+line.H || card.Rect.Y+card.Rect.H > layout.Caret(10).Y {
		t.Fatalf("card %+v, line %+v, next at %v", card.Rect, line, layout.Caret(10).Y)
	}
	// The caret in the source is on the card, above its rule; the set
	// formula beneath it is centered.
	for _, at := range []model.UTF16Offset{3, 4, 6} {
		if caret := layout.Caret(at); !card.Rect.Contains(caret.X+1, caret.Y+1) || caret.Y+caret.H > card.Rule+0.5 {
			t.Fatalf("caret %d = %+v off the source of %+v", at, caret, card)
		}
	}
	if set := card.Set.Rect; set.Y < card.Rule || set.X+set.W/2 < 199 || set.X+set.W/2 > 201 {
		t.Fatalf("set %+v is not centered beneath the rule at %v", set, card.Rule)
	}
	if rects := layout.RangeRects(model.NewUTF16Range(3, 5)); len(rects) != 1 || rects[0].Y < card.Rect.Y {
		t.Fatalf("selection in the source = %+v", rects)
	}
	if got := layout.NextBoundary(6); got != 7 {
		t.Fatalf("right from the source's end = %d", got)
	}
	if got := layout.PreviousBoundary(3); got != 2 {
		t.Fatalf("left from the source's start = %d", got)
	}
	if got := layout.NextBoundary(2); got != 7 {
		t.Fatalf("right over the formula = %d", got)
	}
	caret := layout.Caret(4)
	if got := layout.Hit(ui.Point{X: caret.X, Y: caret.Y + caret.H/2}); got != 4 {
		t.Fatalf("hit in the source = %d", got)
	}
	if got := layout.Hit(ui.Point{X: 5, Y: card.Set.Rect.Y + 1}); got != 6 {
		t.Fatalf("hit on what it sets = %d", got)
	}
	if got := layout.RowNear(4, 1, caret.X); got < 10 {
		t.Fatalf("down from the source = %d", got)
	}
	if got := layout.RowNear(4, -1, 0); got > 2 {
		t.Fatalf("up from the source = %d", got)
	}
	if _, ok := layout.FormulaAt(ui.Point{X: card.Set.Rect.X + 1, Y: card.Set.Rect.Y + 1}); ok {
		t.Fatal("what a card sets is pressed as a formula in its place")
	}
}

// A formula block being edited opens its card around its own line, its
// source above it.
func TestFormulaBlockEditsOnItsCard(t *testing.T) {
	text := "$$\n\\frac12\n$$"
	block := surface.Formula{
		Range: model.NewUTF16Range(0, 13), Text: model.NewUTF16Range(3, 10),
		Source: "\\frac12", Display: true, Open: true, Card: 1,
	}
	layout := NewCache().Update(formulaFlow(t, text, block), 400, testStyle())
	cards, set := layout.FormulaCards(), layout.Formulas()
	if len(cards) != 1 || cards[0].HasSet || len(set) != 1 {
		t.Fatalf("cards = %+v, set %+v", cards, set)
	}
	card := cards[0]
	if card.Rect.W != 400 || set[0].Rect.Y < card.Rule || set[0].Rect.Y+set[0].Rect.H > card.Rect.Y+card.Rect.H {
		t.Fatalf("card %+v does not hold its line %+v below its source", card, set[0].Rect)
	}
	if caret := layout.Caret(10); caret.Y < card.Rect.Y || caret.Y+caret.H > card.Rule+0.5 {
		t.Fatalf("caret at the source's end %+v, card %+v", caret, card)
	}
	if got := layout.NextBoundary(10); got != 13 {
		t.Fatalf("right from the source's end = %d", got)
	}
}

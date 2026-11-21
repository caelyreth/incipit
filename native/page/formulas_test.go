package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// mathPage opens a math block holding src after a paragraph and leaves
// the caret in its source.
func mathPage(t *testing.T, src string) (*Element, func() *unitLayout) {
	t.Helper()
	el := newElement(t)
	typeText(el, "above")
	el.Editor.Do(vocab.Break{}, time.Unix(200, 0))
	typeText(el, "$$")
	typeText(el, src)
	block := func() *unitLayout {
		for _, u := range el.relayout().units {
			if u.pres.Kind == profile.Math {
				return u
			}
		}
		t.Fatal("no math block")
		return nil
	}
	return el, block
}

// A math block shows its formula set for display; its source panel opens
// under it while the caret is in it, and folds away when the caret leaves.
func TestAMathBlockFoldsItsSourceAway(t *testing.T) {
	el, block := mathPage(t, `\frac{a}{b}`)
	u := block()
	if u.math == nil || u.math.box == nil {
		t.Fatal("the block's formula is not set")
	}
	if !u.math.open || u.h != u.math.full || u.math.full <= u.math.fold {
		t.Fatalf("held: height %v, fold %v, full %v", u.h, u.math.fold, u.math.full)
	}
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(300, 0))
	if u = block(); u.math.open || u.h != u.math.fold {
		t.Fatalf("left: open %v, height %v, fold %v", u.math.open, u.h, u.math.fold)
	}
}

// A math block that has never set is its source panel alone, open
// whether held or not; once it has, a source half typed shows the last
// that set, dimmed, its room unchanged.
func TestAMathBlockShowsOnlyMathThatSets(t *testing.T) {
	for _, src := range []string{"", `\`, `\frac{`} {
		el, block := mathPage(t, src)
		if u := block(); u.math.box != nil || u.h != u.math.full {
			t.Fatalf("%q: has a display, or its panel is closed", src)
		}
		el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(300, 0))
		if u := block(); u.math.box != nil || u.h != u.math.full {
			t.Fatalf("%q left: has a display, or its panel is closed", src)
		}
	}
	el, block := mathPage(t, `x^2`)
	sound := block()
	if sound.math.box == nil || sound.math.stale {
		t.Fatal("x^2 is not set")
	}
	h := sound.math.h
	typeText(el, `+\frac{`)
	if u := block(); u.math.box == nil || !u.math.stale || u.math.h != h {
		t.Fatal("a half-typed source does not hold the last that set")
	}
}

// An inline formula sits on its line as an object as wide as its ink.
func TestInlineMathIsAnObjectInItsLine(t *testing.T) {
	el := newElement(t)
	typeText(el, "f $x^2$ g")
	s := el.Editor.layout.units[0].surfaces[0]
	for i, sg := range s.segs {
		if sg.kind != segBox {
			continue
		}
		r, ok := s.objectRect(i)
		if sg.math.box == nil || !ok {
			t.Fatal("the formula is not set as an object")
		}
		if w := sg.math.to - sg.math.from + 2*mathPad; r.W != w || r.H <= 0 {
			t.Fatalf("object %v, want width %v", r, w)
		}
		return
	}
	t.Fatal("no formula")
}

// A math block keeps its display's room as its first math sets, so a
// simple formula appearing shifts nothing below it.
func TestAMathBlockKeepsItsRoomAsMathSets(t *testing.T) {
	el, block := mathPage(t, "")
	h := block().h
	typeText(el, `\frac{a}{b}`)
	if u := block(); u.math.box == nil || u.h != h {
		t.Fatalf("height %v after setting, %v before", u.h, h)
	}
}

// An empty inline formula stands as its hint, and its card fits its text
// under it rather than spanning the column.
func TestAFormulasCardSitsUnderIt(t *testing.T) {
	el := newElement(t)
	typeText(el, "Let us say $")
	u := el.Editor.layout.units[0]
	main, card := u.surfaces[0], u.surfaces[1]
	for i, sg := range main.segs {
		if sg.inline == nil {
			continue
		}
		r, _ := main.objectRect(i)
		if sg.math.box != nil || r.W <= mathPad*2 {
			t.Fatalf("empty formula: math %v, object %v", sg.math.box != nil, r)
		}
		if card.x != r.X || card.w >= el.Editor.metrics.Width/2 {
			t.Fatalf("card at %v, %v wide; formula at %v", card.x, card.w, r.X)
		}
		return
	}
	t.Fatal("no formula")
}

// A formula's hint is shaped with its layout, not as it is painted: an
// empty inline formula's as it is laid out, an empty math block's once it
// is first drawn, and both kept frame after frame.
func TestFormulaHintsAreShapedOnce(t *testing.T) {
	el := newElement(t)
	typeText(el, "Let us say $")
	var hint *ui.TextLayout
	for _, sg := range el.Editor.layout.units[0].surfaces[0].segs {
		if sg.inline != nil {
			hint = sg.shaped
		}
	}
	if hint == nil {
		t.Fatal("an empty formula's hint was not shaped with its layout")
	}

	el, block := mathPage(t, "")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(500 * time.Millisecond)
	first := block().math.hint
	tt.Advance(500 * time.Millisecond)
	if first == nil || block().math.hint != first {
		t.Fatal("an empty math block's hint was shaped again as it was painted")
	}
}

func TestMathAndInlineFormulaDialectPillsSwitchToTypst(t *testing.T) {
	clickPill := func(el *Element, unit doc.ID) {
		t.Helper()
		for _, c := range el.controls() {
			if c.key.unit == unit && c.key.part == partDialect && c.press != nil {
				cx, cy := c.rect.X+c.rect.W/2, c.rect.Y+c.rect.H/2
				if !el.press(cx, cy, 1, false, time.Unix(400, 0)) || !el.letGo(false, time.Unix(401, 0)) {
					t.Fatal("dialect pill did not accept click")
				}
				return
			}
		}
		t.Fatalf("missing dialect pill control for unit %v", unit)
	}

	// Display math block: click TeX -> Typst pill and render Typst syntax.
	el, block := mathPage(t, "")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(500 * time.Millisecond)
	u := block()
	clickPill(el, u.pres.Unit)
	tt.Frame()
	if got := profile.MathDialectOf(block().pres.Node); got != profile.DialectTypst {
		t.Fatalf("math block dialect after click = %v, want Typst", got)
	}
	typeText(el, "frac(a, b)")
	tt.Frame()
	u = block()
	if u.math.dialect != profile.DialectTypst || u.math.box == nil || u.math.stale {
		t.Fatalf("Typst math block did not render soundly: %+v", u.math)
	}

	// Inline formula card: click TeX -> Typst pill inside open card.
	iel := newElement(t)
	typeText(iel, "Let ")
	typeText(iel, "$")
	itt := ui.NewTester(iel.View, 900, 600)
	itt.Advance(500 * time.Millisecond)
	iu := iel.Editor.layout.units[0]
	clickPill(iel, iu.pres.Unit)
	itt.Frame()
	iu = iel.Editor.layout.units[0]
	if len(iu.surfaces) < 2 || !iu.surfaces[1].shown {
		t.Fatalf("expected open inline card after switching dialect, got %d surfaces", len(iu.surfaces))
	}
	if got := profile.MathDialectOf(inlineFormulaNode(iu, iu.surfaces[1].anchor)); got != profile.DialectTypst {
		t.Fatalf("inline formula dialect = %v, want Typst", got)
	}
	typeText(iel, "frac(a, b)")
	itt.Frame()
	iu = iel.Editor.layout.units[0]
	var rendered bool
	for _, sg := range iu.surfaces[0].segs {
		if sg.inline != nil && sg.math.box != nil && !sg.math.stale {
			rendered = true
		}
	}
	if !rendered {
		t.Fatal("inline Typst formula did not render soundly")
	}
}

package page

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/egoist/mygo/ui"
)

func TestFormulaMovesAsOneBoxAtFractionalOrigins(t *testing.T) {
	box := setFormula(`\frac{1}{2}+\sqrt{x^2+y^2}`, profile.DialectTeX, false, 17, 0)
	if box == nil {
		t.Fatal("The math font should set the formula.")
	}
	for _, scale := range []float32{1, 2} {
		dx := float32(0)
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Draw(func(p *ui.Painter, _ ui.Rect) {
				inks := formulaInks{Text: ui.RGB(220, 220, 220)}
				drawFormula(p, box, ui.Point{X: 24 + dx, Y: 48}, inks)
				p.Transform(ui.Translation(dx, 0), 0, 0, func() {
					drawFormula(p, box, ui.Point{X: 324, Y: 48}, inks)
				})
			})
		}, 600, 80)
		tester.SetScale(scale)
		for step := range 8 {
			dx = float32(step) / 8
			tester.Frame()
			img := tester.Image()
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < int(280*scale); x++ {
					a, b := img.RGBAAt(x, y), img.RGBAAt(x+int(300*scale), y)
					for _, delta := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
						if delta < -2 || delta > 2 {
							t.Fatalf("At %v× scale, moving the formula %v DIP changes its ink at (%d, %d).", scale, dx, x, y)
						}
					}
				}
			}
		}
	}
}

// A source's problems are told once each, in order, where they are: a
// command it does not know as such, not as the glyph it has none for.
func TestProblemsAreToldOnceWhereTheyAre(t *testing.T) {
	if got := Problems(`x+\frac12`, false); len(got) != 0 || !formulaSound(`x+\frac12`, profile.DialectTeX, false) {
		t.Fatalf("a sound source has %+v", got)
	}
	if formulaSound("  ", profile.DialectTeX, false) {
		t.Fatal("a blank source is sound")
	}
	got := Problems(`\fr`, false)
	if len(got) != 1 || got[0].From != 0 || got[0].To != 3 || got[0].Message != `undefined control sequence: \fr` {
		t.Fatalf(`\fr has %+v`, got)
	}
	got = Problems(`\frac{1}{2`, false)
	if len(got) != 2 || got[0].From > got[1].From || got[1].From != got[1].To {
		t.Fatalf("an unclosed group has %+v", got)
	}
}

func TestTypstDialectTypesetsSoundlyAndSeparatesCache(t *testing.T) {
	src := "frac(1, 2) + sqrt(x^2 + y^2)"
	if box := setFormula(src, profile.DialectTypst, false, 17, 0); box == nil {
		t.Fatal("Typst formula should produce a rendered box")
	}
	if !formulaSound(src, profile.DialectTypst, false) {
		t.Fatalf("Typst formula has unexpected problems: %+v", ProblemsFor(src, profile.DialectTypst, false))
	}
	if formulaSound("frac(1,", profile.DialectTypst, false) || len(ProblemsFor("frac(1,", profile.DialectTypst, false)) == 0 {
		t.Fatal("unclosed Typst call should report problems")
	}
	// Cache must key by dialect so the same source does not leak across TeX/Typst.
	texSrc := `\frac{1}{2}`
	if !formulaSound(texSrc, profile.DialectTeX, false) {
		t.Fatalf("TeX source failed: %+v", ProblemsFor(texSrc, profile.DialectTeX, false))
	}
	if formulaSound(texSrc, profile.DialectTypst, false) || len(ProblemsFor(texSrc, profile.DialectTypst, false)) == 0 {
		t.Fatalf("expected TeX-only syntax to be diagnosed under Typst instead of hitting TeX cache")
	}
}

func TestFormulaSharesBakedPathsAndUpdatesInkColor(t *testing.T) {
	boxA := setFormula(`x^2 + 1`, profile.DialectTeX, false, 17, 0)
	boxB := setFormula(`x^2 + 2`, profile.DialectTeX, false, 17, 0)
	if boxA == nil || boxB == nil {
		t.Fatal("formulas should typeset cleanly")
	}
	drawA := cachedFormulaDrawing(boxA)
	drawB := cachedFormulaDrawing(boxB)
	seenA := make(map[*ui.BakedPath]struct{})
	for _, op := range drawA.ops {
		if !op.rule && op.baked != nil {
			seenA[op.baked] = struct{}{}
		}
	}
	shared := 0
	for _, op := range drawB.ops {
		if !op.rule && op.baked != nil {
			if _, ok := seenA[op.baked]; ok {
				shared++
			}
		}
	}
	if shared == 0 {
		t.Fatalf("expected distinct formulas with matching glyph sizes to share *ui.BakedPath pointers")
	}

	renderWithInk := func(ink ui.Color) (hasRed, hasGreen int) {
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Draw(func(p *ui.Painter, _ ui.Rect) {
				drawFormula(p, boxA, ui.Point{X: 20, Y: 40}, formulaInks{Text: ink})
			})
		}, 160, 80)
		tt.Frame()
		img := tt.Image()
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				px := img.RGBAAt(x, y)
				if px.R > 150 && px.G < 60 && px.B < 60 {
					hasRed++
				}
				if px.G > 150 && px.R < 60 && px.B < 100 {
					hasGreen++
				}
			}
		}
		return hasRed, hasGreen
	}

	redR, redG := renderWithInk(ui.RGB(220, 40, 40))
	if redR == 0 || redG != 0 {
		t.Fatalf("red ink pass rendered red=%d green=%d", redR, redG)
	}
	greenR, greenG := renderWithInk(ui.RGB(40, 200, 80))
	if greenG == 0 || greenR != 0 {
		t.Fatalf("green ink pass rendered red=%d green=%d", greenR, greenG)
	}
}

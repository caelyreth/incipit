package page

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFormulaMovesAsOneBoxAtFractionalOrigins(t *testing.T) {
	box := setFormula(`\frac{1}{2}+\sqrt{x^2+y^2}`, false, 17, 0)
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
	if got := Problems(`x+\frac12`, false); len(got) != 0 || !formulaSound(`x+\frac12`, false) {
		t.Fatalf("a sound source has %+v", got)
	}
	if formulaSound("  ", false) {
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

package page

import (
	"testing"

	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Moving a text origin must show the same glyphs as moving the whole
// resting line. Per-glyph pixel snapping changes the spaces between them.
func TestTextMovesAsOneLineAtFractionalOrigins(t *testing.T) {
	layout := ui.ShapeText("Sphinx 0123456789 fi", ui.Font{Family: theme.UIFontFamily, Size: 17}, 0)
	for _, scale := range []float32{1, 2} {
		dx := float32(0)
		tester := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Draw(func(p *ui.Painter, _ ui.Rect) {
				ink := ui.RGB(220, 220, 220)
				paintText(p, layout, 24+dx, 24, ink)
				p.Transform(ui.Translation(dx, 0), 0, 0, func() {
					p.TextLayout(layout, 324, 24, ink)
				})
			})
		}, 600, 80)
		tester.SetScale(scale)
		for step := range 8 {
			dx = float32(step) / 8
			tester.Frame()
			img := tester.Image()
			offset := int(300 * scale)
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < int(280*scale); x++ {
					a, b := img.RGBAAt(x, y), img.RGBAAt(x+offset, y)
					for _, delta := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
						if delta < -2 || delta > 2 {
							t.Fatalf("At %v× scale, origin moved %v DIP differs from a translated resting line at pixel (%d, %d): %v versus %v.",
								scale, dx, x, y, a, b)
						}
					}
				}
			}
		}
	}
}

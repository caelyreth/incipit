package components

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestElevatedSurfacePreservesMaterialVariants(t *testing.T) {
	for _, appearance := range []theme.Appearance{theme.Starfield, theme.Daybreak} {
		for _, scale := range []float32{12, 16, 24} {
			for _, elevation := range []Elevation{PanelElevation, PopoverElevation} {
				t.Run(fmt.Sprintf("%d-%v-%d", appearance, scale, elevation), func(t *testing.T) {
					settings := theme.DefaultSettings()
					settings.Appearance = appearance
					semantic := settings.Semantic()
					tester := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(settings.MyGoTheme())
						ui.Row(c).Fill().Padding(40).Gap(50).Children(func() {
							ElevatedSurface(ui.Box(c).Size(90, 50), semantic, scale, elevation)
							panel := ui.Box(c).Size(90, 50)
							y, blur, alpha := 4*scale/theme.DefaultUIFontSize, 12*scale/theme.DefaultUIFontSize, float32(0.25)
							if elevation == PopoverElevation {
								y, blur, alpha = 8, 24, 0.28
							}
							panel.Radius(11*scale/theme.DefaultUIFontSize).
								Border(1, semantic.Border.Default).
								Background(semantic.Surface.Elevated).
								Shadow(0, y, blur, 0, ui.RGBA(0, 0, 0, alpha))
						})
					}, 320, 130)
					tester.Frame()
					image := tester.Image()
					for y := 20; y < 120; y++ {
						for x := 20; x < 140; x++ {
							first := image.RGBAAt(x, y)
							second := image.RGBAAt(x+140, y)
							if first != second {
								t.Fatalf("material differs at %d,%d: %v != %v", x, y, first, second)
							}
						}
					}
				})
			}
		}
	}
}

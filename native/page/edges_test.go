package page

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func referenceEdgeMask(top, bottom, height float32) ui.LinearGradient {
	ease := [...]float32{0, 0.156, 0.5, 0.844, 1}
	var stops []ui.GradientStop
	for index, alpha := range ease {
		stops = append(stops, ui.GradientStop{Color: ui.RGB(0, 0, 0).Alpha(alpha), At: top / height * float32(index) / 4})
	}
	for index := len(ease) - 1; index >= 0; index-- {
		stops = append(stops, ui.GradientStop{Color: ui.RGB(0, 0, 0).Alpha(ease[index]), At: 1 - bottom/height*float32(index)/4})
	}
	return ui.LinearGradient{Angle: 180, Stops: stops}
}

func TestEdgeFadesMatchGradientMasks(t *testing.T) {
	for _, appearance := range []theme.Appearance{theme.Daybreak, theme.Starfield} {
		for _, scale := range []float32{1, 2} {
			for _, height := range []int{200, 263} {
				t.Run(fmt.Sprint(appearance, "/", scale, "/", height), func(t *testing.T) {
					el := newElement(t)
					settings := theme.DefaultSettings()
					settings.Appearance = appearance
					el.Theme = settings.Semantic()
					el.Top, el.Bottom, el.Dock = 64, 48, 48
					top, bottom := el.edgeFades()
					draw := func(p *ui.Painter, view ui.Rect) {
						p.Fill(ui.Rect{X: view.X + 8, Y: view.Y, W: 24, H: view.H}, el.Theme.Accent.Foreground, 0)
						p.Fill(ui.Rect{X: view.X + 40, Y: view.Y, W: 24, H: view.H}, el.Theme.Prose.Selection, 0)
						p.Text(view.X+72, view.Y+8, "Title", 19, el.Theme.Text.Default)
						p.Text(view.X+72, view.Y+view.H-28, "Text", 19, el.Theme.Text.Default)
					}
					tester := ui.NewTester(func(c *ui.Context) {
						ui.Box(c).Fill().Background(el.Theme.Surface.Page).Children(func() {
							ui.Box(c).Absolute().Left(0).Top(0).Width(160).Height(float32(height)).
								MaskLinearGradient(referenceEdgeMask(top, bottom, float32(height))).Draw(draw)
							ui.Box(c).Absolute().Left(160).Top(0).Width(160).Height(float32(height)).
								Draw(draw).DrawOver(el.paintEdges)
						})
					}, 320, height)
					tester.SetScale(scale)
					img := tester.Image()
					offset := int(160 * scale)
					textDifference := 0
					for row := range img.Bounds().Dy() {
						for column := range offset {
							before, after := img.RGBAAt(column, row), img.RGBAAt(column+offset, row)
							for _, delta := range []int{int(before.R) - int(after.R), int(before.G) - int(after.G), int(before.B) - int(after.B)} {
								if column >= int(72*scale) {
									textDifference = max(textDifference, delta, -delta)
									continue
								}
								if delta < -3 || delta > 3 {
									t.Fatalf("The edge fade changed at (%d,%d): %v versus %v.", column, row, before, after)
								}
							}
						}
					}
					if textDifference > 6 {
						t.Fatalf("The edge fade changed text coverage by %d/255.", textDifference)
					}
				})
			}
		}
	}
}

package settings

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestSettingsTagFramePreservesGeometry(t *testing.T) {
	for _, scale := range []float32{12, 16, 24} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Row(c).Fill().Padding(20).Gap(40).Children(func() {
					ui.Box(c).Width(100).Children(func() {
						settingsTagFrame(ui.Row(c), scale).Background(ui.RGB(100, 120, 140)).Children(func() {
							ui.Text(c, "Custom")
						})
					})
					ui.Box(c).Width(100).Children(func() {
						ui.Row(c).Shrink(0).Height(18 * scale / theme.DefaultUIFontSize).AlignItems(ui.Center).
							PaddingX(theme.Space06.DIPs(scale)).Radius(9 * scale / theme.DefaultUIFontSize).
							Background(ui.RGB(100, 120, 140)).Children(func() {
							ui.Text(c, "Custom")
						})
					})
				})
			}, 300, 100)
			tester.Frame()
			image := tester.Image()
			for y := 0; y < 80; y++ {
				for x := 20; x < 120; x++ {
					if left, right := image.RGBAAt(x, y), image.RGBAAt(x+140, y); left != right {
						t.Fatalf("tag differs at %d,%d: %v != %v", x, y, left, right)
					}
				}
			}
		})
	}
}

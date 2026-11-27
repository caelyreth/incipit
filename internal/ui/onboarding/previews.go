package onboarding

import (
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// previewHeight is how tall an answer's preview stands, at the default UI
// size.
const previewHeight = 96

// themePreview is a small page in the theme chosen: the window's ground,
// a sheet on it with a heading and a line of prose. The System theme's is
// the two halves, night and day, side by side.
func themePreview(c *ui.Context, m metrics, choice appsettings.ThemeChoice, family string) {
	if choice != appsettings.System {
		miniPage(c, m, paletteOf(choice), family, true, true)
		return
	}
	ui.Row(c).WidthPercent(100).Children(func() {
		ui.Box(c).Grow(1).Basis(0).Clip().Children(func() {
			miniPage(c, m, paletteOf(appsettings.Starfield), family, true, false)
		})
		ui.Box(c).Grow(1).Basis(0).Clip().Children(func() {
			miniPage(c, m, paletteOf(appsettings.Daybreak), family, false, true)
		})
	})
}

// paletteOf is choice's colors, a fixed theme's.
func paletteOf(choice appsettings.ThemeChoice) theme.Theme {
	settings := theme.DefaultSettings()
	settings.Appearance = theme.Appearance(choice)
	return settings.Semantic()
}

// miniPage is a sheet on the ground of semantic, its left or right edge
// cut where a split preview halves it.
func miniPage(c *ui.Context, m metrics, semantic theme.Theme, family string, left, right bool) {
	pad := 10 * m.k
	ui.Box(c).WidthPercent(100).Height(previewHeight*m.k).Background(semantic.Surface.Background).
		Padding(pad, rightPad(right, pad), 0, leftPad(left, pad)).Children(func() {
		radius := 6 * m.k
		ui.Column(c).WidthPercent(100).HeightPercent(100).Background(semantic.Surface.Page).
			Radius(cornerRadii(left, right, radius)...).Border(1, semantic.Border.Variant).
			Padding(10*m.k, 12*m.k).Gap(5 * m.k).Children(func() {
			ui.Text(c, locales.Notebook()).Font(family).FontSize(13 * m.k).FontWeight(600).
				TextColor(semantic.Text.Heading).SingleLine()
			ui.Text(c, locales.OnboardingSample()).Font(family).FontSize(11 * m.k).LineHeight(1.5).
				TextColor(semantic.Text.Default).MaxLines(2)
		})
	})
}

func leftPad(on bool, pad float32) float32 {
	if on {
		return pad
	}
	return 0
}

func rightPad(on bool, pad float32) float32 { return leftPad(on, pad) }

// cornerRadii rounds the sheet's corners on the sides it keeps.
func cornerRadii(left, right bool, r float32) []float32 {
	l, rr := leftPad(left, r), leftPad(right, r)
	return []float32{l, rr, 0, 0}
}

// facePreview sets a large pair of letters and the sample line in family.
func facePreview(c *ui.Context, m metrics, semantic theme.Theme, family string) {
	ui.Row(c).WidthPercent(100).Height(previewHeight*m.k).Background(semantic.Surface.Page).
		Padding(0, 16*m.k).Gap(14 * m.k).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Ag").Font(family).FontSize(46 * m.k).FontWeight(500).
			TextColor(semantic.Accent.Foreground).Shrink(0)
		ui.Text(c, locales.OnboardingSample()).Font(family).FontSize(13 * m.k).LineHeight(1.5).
			TextColor(semantic.Text.Default).Grow(1).MinWidth(0).MaxLines(3)
	})
}

// menuPreview draws a menu as it would open over a page: the Mac's own, in its material and its blue, or Incipit's, of
// the page's surfaces with a wash of its ink.
func menuPreview(c *ui.Context, m metrics, semantic theme.Theme, drawn bool) {
	ui.Box(c).WidthPercent(100).Height(previewHeight * 1.05 * m.k).Background(semantic.Surface.Page).Clip().Children(func() {
		// Lines of a page behind it.
		ui.Column(c).Absolute().Left(14 * m.k).Top(14 * m.k).Right(14 * m.k).Gap(9 * m.k).Children(func() {
			for _, w := range []float32{92, 78, 86, 60} {
				ui.Box(c).WidthPercent(w).Height(5 * m.k).Radius(3 * m.k).Background(semantic.Text.Muted.Alpha(0.22))
			}
		})
		items := []string{locales.Open(), locales.OpenAlongside(), "", locales.Duplicate()}
		panel := ui.Column(c).Absolute().Left(34 * m.k).Top(10 * m.k).Width(150 * m.k).Padding(5 * m.k)
		dark := semantic.Appearance == theme.Starfield
		if drawn {
			components.ElevatedSurface(panel, semantic, m.scale, components.PopoverElevation).Border(components.Rim, semantic.Text.Default.Alpha(0.1))
		} else {
			material := ui.RGBA(246, 246, 246, 0.94)
			rim := ui.RGBA(0, 0, 0, 0.12)
			if dark {
				material, rim = ui.RGBA(50, 50, 52, 0.94), ui.RGBA(255, 255, 255, 0.14)
			}
			panel.Background(material).Radius(8*m.k).Border(0.5, rim).Shadow(0, 8*m.k, 24*m.k, 0, ui.RGBA(0, 0, 0, 0.28))
		}
		panel.Children(func() {
			for i, label := range items {
				if label == "" {
					rule := semantic.Text.Default.Alpha(0.12)
					if !drawn {
						rule = ui.RGBA(128, 128, 128, 0.3)
					}
					ui.Box(c).WidthPercent(100).Height(1).Margin(4*m.k, 0).Background(rule)
					continue
				}
				ink := semantic.Text.Default
				row := ui.Row(c).WidthPercent(100).Padding(3*m.k, 8*m.k).Radius(5 * m.k)
				if !drawn {
					ink = ui.RGB(30, 30, 30)
					if dark {
						ink = ui.RGB(235, 235, 235)
					}
				}
				if i == 1 {
					if drawn {
						row.Background(semantic.Text.Default.Alpha(0.1))
					} else {
						row.Background(ui.RGB(0, 100, 225))
						ink = ui.RGB(255, 255, 255)
					}
				}
				row.Children(func() {
					ui.Text(c, label).Font(theme.UIFontFamily).FontSize(12 * m.k).TextColor(ink).SingleLine()
				})
			}
		})
	})
}


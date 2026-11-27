package components

import (
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Menus has the window show its popup menus as Incipit does on its
// platform (menuPresentation): the system's on macOS unless drawn is
// chosen there, elsewhere drawn in flyouts of the window, as macOS's
// stand: the raised surface over the system's material, letting a little
// of it through (menuOverMaterial), a faint light rim, the row under the
// pointer washed in the words' ink.
func Menus(c *ui.Context, semantic theme.Theme, drawn bool) {
	presentation := menuPresentation
	if drawn {
		presentation = ui.FlyoutMenus
	}
	c.SetMenuPresentation(presentation)
	if presentation == ui.SystemMenus {
		return
	}
	scale := UIFontSize(c)
	controls := theme.DefaultControls.Scaled(scale)
	// The words a step under a row's, as a menu's are under a list's; the
	// rows a line high and a little more, rounded within the panel's
	// corners, a few points in from them. Across, the theme's padding,
	// which a palette's swatches line up by.
	size, line := menuText*scale/theme.DefaultUIFontSize, theme.RowTitle.LineHeight(scale)
	inset := theme.Space04.DIPs(scale) + 1
	pad := theme.Space02.DIPs(scale)
	across := 2 * c.Theme().Spacing
	// The rim, the lines between groups and the wash are the words' ink,
	// faint: light on a dark menu, dark on a light one, as the system's.
	ink := semantic.Text.Default
	rim, rule, wash := ink.Alpha(0.1), ink.Alpha(0.12), ink.Alpha(0.1)
	c.SetMenuStyle(ui.PopoverMenuOptions{
		Panel: func(panel ui.Element) {
			ElevatedSurface(panel, semantic, scale, PopoverElevation).Padding(inset).Border(Rim, rim)
		},
		Item: func(item ui.Element) {
			item.Radius(controls.PanelRadius-inset).FontSize(size).FixedLineHeight(line).Padding(pad, across)
			if item.Highlighted() {
				item.Background(wash)
			}
		},
		Highlight:    ui.SurfaceHighlight,
		Radius:       controls.PanelRadius,
		Separator:    rule,
		OverMaterial: menuOverMaterial,
	})
}

// menuOverMaterial is how much of the raised surface a menu stands on over
// the system's material behind its flyout: most of it. The material blurs
// what is behind, which over a dark page of text is a flat grey streaked
// with its lines; let through by half, as by default, the menu reads as
// that grey, lighter than what is raised and smudged. A little of it keeps
// the frost over the desktop and the field's colors.
const menuOverMaterial = 0.85

// menuText is the size of a menu's words at the default UI size, as
// macOS's menus set theirs.
const menuText = 13

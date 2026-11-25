package components

import (
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Menus has the window show its popup menus as Neverland does on its
// platform (menuPresentation): the system's on macOS unless drawn is
// chosen there, elsewhere drawn in flyouts of the window, as macOS's
// stand: the raised surface laid thin over the system's material, frosted
// glass where the system has it (menuGlass), a faint light rim, the row
// under the pointer washed in the words' ink.
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
			ElevatedSurface(panel, semantic, scale, PopoverElevation).Padding(inset).Border(Rim, rim).
				Background(semantic.Surface.Elevated.Alpha(menuGlass))
		},
		Item: func(item ui.Element) {
			item.Radius(controls.PanelRadius-inset).FontSize(size).FixedLineHeight(line).Padding(pad, across)
			if item.Highlighted() {
				item.Background(wash)
			}
		},
		Highlight: ui.SurfaceHighlight,
		Radius:    controls.PanelRadius,
		Separator: rule,
	})
}

// menuText is the size of a menu's words at the default UI size, as
// macOS's menus set theirs.
const menuText = 13

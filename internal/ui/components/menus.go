package components

import (
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Menus has the window show its popup menus as Neverland does on its
// platform (menuPresentation): the system's on macOS, elsewhere drawn in
// flyouts of the window, in the raised surface of its popovers, the row
// under the pointer lit as its lists light theirs.
func Menus(c *ui.Context, semantic theme.Theme) {
	c.SetMenuPresentation(menuPresentation)
	if menuPresentation == ui.SystemMenus {
		return
	}
	scale := UIFontSize(c)
	controls := theme.DefaultControls.Scaled(scale)
	size, line := theme.RowTitle.Size(scale), theme.RowTitle.LineHeight(scale)
	// As macOS's menus stand: a panel a few points in from its rows, over
	// the system's material where it has one, the rows a line high and a
	// little more, rounded within the panel's corners. Across, the theme's
	// padding, which a palette's swatches line up by.
	inset := theme.Space04.DIPs(scale) + 1
	pad := theme.Space02.DIPs(scale)
	across := 2 * c.Theme().Spacing
	// Its rim half as strong as a popover's: the material and the shadow
	// set it apart.
	rim := semantic.Border.Default
	rim.A /= 2
	c.SetMenuStyle(ui.PopoverMenuOptions{
		Panel: func(panel ui.Element) {
			ElevatedSurface(panel, semantic, scale, PopoverElevation).Padding(inset).Border(Rim, rim)
		},
		Item: func(item ui.Element) {
			item.Radius(controls.PanelRadius-inset).FontSize(size).FixedLineHeight(line).Padding(pad, across)
		},
		Highlight: ui.SurfaceHighlight,
		Radius:    controls.PanelRadius,
	})
}

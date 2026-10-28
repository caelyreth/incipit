package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) iconButton(
	c *ui.Context,
	key string,
	label string,
	icon Icon,
	metrics theme.ControlMetrics,
	semantic theme.Theme,
) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).
		Size(metrics.Height, metrics.Height).Padding(0).Radius(metrics.Radius).
		Background(ui.Transparent)
	if button.Hovered() {
		if button.Pressed() {
			button.Background(semantic.Element.Selected)
		} else {
			button.Background(semantic.Element.Hover)
		}
	}
	button.Children(func() {
		ui.Icon(c, glyph(icon)).FontSize(metrics.Icon).TextColor(semantic.Icon.Default)
	})
	return button
}

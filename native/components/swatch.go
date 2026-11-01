package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type Swatch struct {
	Key      string
	Label    string
	Color    ui.Color
	Selected bool
	Action   func()
}

func (s Swatch) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	size := 22 * scale / theme.DefaultUIFontSize
	ring := ui.Transparent
	if s.Selected {
		ring = semantic.Text.Heading
	}
	button := ui.ButtonBase(c.Key(s.Key)).Role(ui.RoleRadio).
		Label(s.Label).Tooltip(s.Label).Selected(s.Selected).
		Size(size, size).Padding(0).Radius(size/2).
		Background(s.Color).Border(2*scale/theme.DefaultUIFontSize, ring)
	if button.Clicked() && s.Action != nil {
		s.Action()
	}
	return button
}

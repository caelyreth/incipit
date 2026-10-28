package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type MenuButton struct {
	Key     string
	Label   string
	Icon    *ui.SVG
	Animate bool
	Build   func(*ui.Menu)
}

func (b MenuButton) View(c *ui.Context, semantic theme.Theme) ui.Element {
	button := IconButton{
		Key: b.Key, Label: b.Label, Icon: b.Icon,
		Style: SubtleButton, Size: CompactButton,
		IconColor: &semantic.Icon.Muted, Animate: b.Animate,
	}.View(c, semantic)
	return button.Menu(b.Build)
}

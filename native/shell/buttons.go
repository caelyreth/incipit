package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// chromeButton is one of the shell's icon buttons over its panes, the
// window's corner and the canvas: subtle, its glyph in the icons' muted
// ink and a little under the controls' own icon size (16 at the reference
// type size for a medium button, 14 for a compact one), at the type size
// around it (components.UIFontSize), so it follows the canvas's zoom. label
// names it; tooltip is what its tooltip says, the label when empty.
type chromeButton struct {
	key, label, tooltip string
	icon                components.IconName
	compact, selected   bool
	action              func()
}

func (b chromeButton) view(c *ui.Context, semantic theme.Theme, animate bool) ui.Element {
	size, glyph := components.MediumButton, float32(16)
	if b.compact {
		size, glyph = components.CompactButton, 14
	}
	tooltip := b.tooltip
	if tooltip == "" {
		tooltip = b.label
	}
	action := b.action
	if action == nil {
		action = func() {}
	}
	return components.IconButton{
		Key: b.key, Label: b.label, Tooltip: tooltip, Icon: b.icon.SVG(),
		Style: components.SubtleButton, Size: size,
		IconSize: glyph * components.UIFontSize(c) / theme.DefaultUIFontSize, IconColor: &semantic.Icon.Muted,
		Selected: b.selected, Animate: animate, Action: action,
	}.View(c, semantic)
}

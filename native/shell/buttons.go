package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// chromeButton is one of the shell's icon buttons over its panes, the
// window's corner and the desk: subtle, its glyph in the icons' muted
// ink and a little under the controls' own icon size (16 at the reference
// type size for a medium button, 14 for a compact one), at the type size
// around it (components.UIFontSize), so it follows the type size. label
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

// laneCircleButton is a round field-lane control button sized size wide.
func laneCircleButton(c *ui.Context, semantic theme.Theme, key string, icon components.IconName, label, tooltip string, selected bool, size float32, animate bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).Label(label).Tooltip(tooltip).Selected(selected)
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	// The place the field shows stands inverse among them.
	on := map[bool]float32{true: 1}[selected]
	if animate {
		on = components.Follow(btn, "selected", on, components.MotionQuick)
	}
	ground := components.Blend(semantic.Surface.Page, semantic.Surface.Inverse, on)
	ink := components.Blend(semantic.Icon.Muted, semantic.Text.OnInverse, on)
	rim := semantic.Border.Variant.Alpha(1 - on)
	btn.Background(components.Blend(ground, semantic.GhostElement.Hover, hover)).Border(1, rim)
	btn.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(size / 2).TextColor(ink)
	})
	return btn
}

// miniCloseButton renders a compact round × icon button with a faded hover
// fill, used on kept bookmarks and query ribbons.
func miniCloseButton(
	c *ui.Context,
	key, label, tooltip string,
	size, glyph float32,
	ink, hoverFill ui.Color,
	animate bool,
	onClick func(),
) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).
		Label(label).Tooltip(tooltip)
	near := components.HoverAmount(btn, btn.Hovered(), animate)
	btn.Background(components.Blend(hoverFill.Alpha(0), hoverFill, near))
	if btn.Clicked() && onClick != nil {
		onClick()
	}
	btn.Children(func() {
		ui.Icon(c, components.IconClose.SVG()).FontSize(glyph).TextColor(ink)
	})
	return btn
}

package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// chromeButton is one of the shell's icon buttons on the desk's column
// heads: subtle, its glyph in the icons' muted ink (16 at the reference
// type size), at the type size around it (components.UIFontSize). label
// names it; tooltip is what its hint says, the label when empty, binding
// its key and side where the hint stands.
type chromeButton struct {
	key, label, tooltip string
	binding             string
	side                components.HintSide
	icon                components.IconName
	action              func()
}

func (b chromeButton) view(c *ui.Context, semantic theme.Theme, animate bool) ui.Element {
	tooltip := b.tooltip
	if tooltip == "" {
		tooltip = b.label
	}
	action := b.action
	if action == nil {
		action = func() {}
	}
	return components.IconButton{
		Key: b.key, Label: b.label, Tooltip: tooltip, Binding: b.binding, HintSide: b.side, Icon: b.icon.SVG(),
		Style: components.SubtleButton, Size: components.MediumButton,
		IconSize: 16 * components.UIFontSize(c) / theme.DefaultUIFontSize, IconColor: &semantic.Icon.Muted,
		Animate: animate, Action: action,
	}.View(c, semantic)
}

// laneCircleButton is a round field-lane control button sized size wide,
// its hint, hint and its key binding, standing past it, into the field.
func laneCircleButton(c *ui.Context, semantic theme.Theme, key string, icon components.IconName, label, hint, binding string, selected bool, size float32, animate bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().Label(label).Selected(selected)
	components.Hint(c, semantic, btn, hint, binding, components.HintAfter)
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
	semantic theme.Theme,
	key, label, tooltip string,
	size, glyph float32,
	ink, hoverFill ui.Color,
	animate bool,
	onClick func(),
) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Size(size, size).Radius(size / 2).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().
		Label(label)
	components.Hint(c, semantic, btn, tooltip, "", components.HintAbove)
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

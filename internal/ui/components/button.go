package components

import (
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

const DisabledOpacity = float32(0.4)

type ButtonStyle uint8

const (
	FilledButton ButtonStyle = iota + 1
	PrimaryButton
	DestructiveButton
	SubtleButton
	OutlinedButton
	// WarningButton is a subtle button in the warning hue: its words carry
	// the color, over no fill, and the hue's surface answers the pointer.
	WarningButton
)

type ButtonSize uint8

const (
	CompactButton ButtonSize = iota + 1
	MediumButton
)

func (s ButtonSize) Height(scale float32) float32 {
	metrics := theme.DefaultControls.Scaled(scale)
	if s == CompactButton {
		return metrics.InlineHeight
	}
	return metrics.Height
}

type Button struct {
	Key      string
	Label    string
	Style    ButtonStyle
	Size     ButtonSize
	Selected bool
	Disabled bool
	Animate  bool
	Fill     *ui.Color
	// Shortcut, when set, is a key the button answers to wherever the
	// window's keys go, its cap shown at the button's end.
	Shortcut ButtonKey
	Action   func()
}

// ButtonKey is a key a Button answers to (Shortcut): pressed while the button can
// act, it acts as a click. Its cap shows after the label.
type ButtonKey struct {
	Mods ui.Modifiers
	Key  ui.Key
	// Cap is the key as its cap names it, as "↩" for Return.
	Cap string
}

func (b Button) View(c *ui.Context, semantic theme.Theme) ui.Element {
	ink := buttonTextColor(semantic, b.Style, b.Selected)
	if b.Shortcut.Key != 0 && !b.Disabled && b.Action != nil && c.Shortcut(b.Shortcut.Mods, b.Shortcut.Key) {
		b.Action()
	}
	scale := UIFontSize(c)
	el := buttonElement(c, semantic, buttonOptions{
		key:      b.Key,
		label:    b.Label,
		style:    b.Style,
		size:     b.Size,
		selected: b.Selected,
		disabled: b.Disabled,
		animate:  b.Animate,
		fill:     b.Fill,
		action:   b.Action,
		children: func() {
			if b.Shortcut.Cap == "" {
				CenteredLabel(c, b.Label, theme.TextSmall, ink)
				return
			}
			ui.Row(c).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
				CenteredLabel(c, b.Label, theme.TextSmall, ink)
				Keycaps(c, buttonKeycap(semantic, scale, b.Style, b.Selected), []string{b.Shortcut.Cap}, "").AccessHidden(true)
			})
		},
	}).PaddingX(theme.Space12.DIPs(scale)).MinWidth(0)
	if b.Shortcut.Key != 0 {
		el.KeyShortcuts(b.Shortcut.Mods, b.Shortcut.Key)
		if b.Shortcut.Cap != "" {
			// The cap stands in a little from the end, as far as the label
			// from the start's rim less the cap's own margin.
			el.Padding(0, theme.Space06.DIPs(scale), 0, theme.Space12.DIPs(scale))
		}
	}
	return el
}

// buttonKeycap is a key's cap inside a button: smaller than a row's, its
// face a step off the button's own and its glyph the label's ink, softer.
func buttonKeycap(semantic theme.Theme, scale float32, style ButtonStyle, selected bool) Keycap {
	k := KeycapStyle(semantic, scale, true)
	unit := scale / theme.DefaultUIFontSize
	k.Height, k.Radius, k.PadX = 18*unit, 4*unit, theme.Space04.DIPs(scale)
	rest, _, _ := buttonColors(semantic, style, selected)
	label := buttonTextColor(semantic, style, selected)
	if rest.A == 0 {
		rest = semantic.Surface.Page
	}
	k.Face, k.Border, k.Ink = blend(rest, label, 0.08), blend(rest, label, 0.22), blend(rest, label, 0.7)
	return k
}

type IconButton struct {
	Key   string
	Label string
	// Tooltip is what its hint says, its Label where empty; Binding its
	// key, and HintSide where the hint stands.
	Tooltip  string
	Binding  string
	HintSide HintSide
	Icon     *ui.SVG
	Style    ButtonStyle
	Size     ButtonSize
	IconSize float32
	// IconColor is its glyph's ink, unset the icons' own, as it is
	// selected the accent's.
	IconColor ui.Color
	Selected  bool
	Disabled  bool
	Animate   bool
	Action    func()
}

func (b IconButton) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	size := b.Size
	if size == 0 {
		size = MediumButton
	}
	height := size.Height(scale)
	iconSize := b.IconSize
	if iconSize <= 0 {
		if size == CompactButton {
			iconSize = 16 * scale / theme.DefaultUIFontSize
		} else {
			iconSize = theme.DefaultControls.Scaled(scale).Icon
		}
	}
	iconColor := semantic.Icon.Default
	if b.Selected {
		iconColor = semantic.Text.OnInverse
	} else if b.IconColor != (ui.Color{}) {
		iconColor = b.IconColor
	}
	hint := b.Tooltip
	if hint == "" {
		hint = b.Label
	}
	return buttonElement(c, semantic, buttonOptions{
		key:      b.Key,
		label:    b.Label,
		tooltip:  hint,
		binding:  b.Binding,
		side:     b.HintSide,
		style:    b.Style,
		size:     size,
		selected: b.Selected,
		disabled: b.Disabled,
		animate:  b.Animate,
		action:   b.Action,
		children: func() {
			ui.Icon(c, b.Icon).FontSize(iconSize).TextColor(iconColor)
		},
	}).Width(height)
}

type buttonOptions struct {
	key      string
	label    string
	tooltip  string
	binding  string
	side     HintSide
	style    ButtonStyle
	size     ButtonSize
	selected bool
	disabled bool
	animate  bool
	fill     *ui.Color
	action   func()
	children func()
}

func buttonElement(
	c *ui.Context,
	semantic theme.Theme,
	options buttonOptions,
) ui.Element {
	if options.style == 0 {
		options.style = FilledButton
	}
	if options.size == 0 {
		options.size = MediumButton
	}
	scale := UIFontSize(c)
	height := options.size.Height(scale)
	rest, hover, border := buttonColors(semantic, options.style, options.selected)
	if options.fill != nil {
		rest = *options.fill
		hover = blend(rest, semantic.Surface.Page, 0.15)
	}
	inverse := rest == semantic.Surface.Inverse

	// A press leaves the keyboard where it is, as a Mac's buttons do: the
	// pill or the page keeps it (only Tab moves it here).
	button := ui.ButtonBase(c.Key(options.key)).Label(options.label).KeepFocus().
		Height(height).Padding(0).Radius(theme.DefaultControls.Scaled(scale).Radius).
		Border(1, border).Disabled(options.disabled)
	// Only a button with no words of its own takes a hint.
	if options.tooltip != "" {
		Hint(c, semantic, button, options.tooltip, options.binding, options.side)
	}
	if options.disabled {
		button.Opacity(DisabledOpacity)
	}
	hoverAmount := HoverAmount(button, !options.disabled && button.Hovered(), options.animate)
	background := blend(rest, hover, hoverAmount)
	if !options.disabled && button.Pressed() {
		// A press is the ink laid over the ground, or, on the inverse, the
		// ground let toward the muted ink.
		if inverse {
			background = blend(background, semantic.Text.Muted, 0.35)
		} else {
			background = blend(background, semantic.GhostElement.Selected, 1)
		}
	}
	button.Background(background)
	button.Children(options.children)
	if !options.disabled && button.Clicked() && options.action != nil {
		options.action()
	}
	return button
}

// buttonColors are a button's fills at rest and under the pointer, and its
// rim. What is chosen, and the primary action, stand on the inverse, as
// the chosen option of a segmented choice does: Moonveil's ink as ground,
// a step softer under the pointer, and no rim.
func buttonColors(
	semantic theme.Theme,
	style ButtonStyle,
	selected bool,
) (rest, hover, border ui.Color) {
	if style == PrimaryButton || selected && style != DestructiveButton {
		return semantic.Surface.Inverse, semantic.Text.Default, ui.Transparent
	}
	switch style {
	case DestructiveButton:
		return semantic.Status.Danger.Foreground, semantic.Status.Danger.Foreground, ui.Transparent
	case SubtleButton:
		return semantic.GhostElement.Background, semantic.GhostElement.Hover, ui.Transparent
	case OutlinedButton:
		return semantic.GhostElement.Background, semantic.GhostElement.Hover, semantic.Border.Variant
	case WarningButton:
		return semantic.GhostElement.Background, semantic.Status.Warning.Background, ui.Transparent
	default:
		return semantic.Element.Background, semantic.Element.Hover, semantic.Border.Default
	}
}

func buttonTextColor(semantic theme.Theme, style ButtonStyle, selected bool) ui.Color {
	if style == PrimaryButton || selected && style != DestructiveButton {
		return semantic.Text.OnInverse
	}
	if style == DestructiveButton {
		return semantic.Accent.OnAccent
	}
	if style == WarningButton {
		return semantic.Status.Warning.Foreground
	}
	return semantic.Text.Default
}

// Blend lays overlay over base at amount, as a hover tint over a rest
// color.
func Blend(base, overlay ui.Color, amount float32) ui.Color { return blend(base, overlay, amount) }

func blend(base, overlay ui.Color, amount float32) ui.Color {
	amount = max(0, min(amount, 1))
	baseAlpha := float32(base.A) / 255
	overlayAlpha := float32(overlay.A) / 255 * amount
	outAlpha := overlayAlpha + baseAlpha*(1-overlayAlpha)
	if outAlpha <= 0 {
		return ui.Transparent
	}
	mix := func(a, b uint8) uint8 {
		value := (float32(b)*overlayAlpha + float32(a)*baseAlpha*(1-overlayAlpha)) / outAlpha
		return uint8(max(0, min(value, 255)))
	}
	return ui.RGBA(
		mix(base.R, overlay.R),
		mix(base.G, overlay.G),
		mix(base.B, overlay.B),
		outAlpha,
	)
}

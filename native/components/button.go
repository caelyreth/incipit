package components

import (
	"github.com/caelyreth/neverland/native/theme"
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
	Tooltip  string
	Style    ButtonStyle
	Size     ButtonSize
	Selected bool
	Disabled bool
	Animate  bool
	Fill     *ui.Color
	Action   func()
}

func (b Button) View(c *ui.Context, semantic theme.Theme) ui.Element {
	return buttonElement(c, semantic, buttonOptions{
		key:      b.Key,
		label:    b.Label,
		tooltip:  b.Tooltip,
		style:    b.Style,
		size:     b.Size,
		selected: b.Selected,
		disabled: b.Disabled,
		animate:  b.Animate,
		fill:     b.Fill,
		action:   b.Action,
		children: func() {
			CenteredLabel(c, b.Label, theme.TextSmall, buttonTextColor(semantic, b.Style))
		},
	}).PaddingX(theme.Space12.DIPs(UIFontSize(c))).MinWidth(0)
}

type IconButton struct {
	Key       string
	Label     string
	Tooltip   string
	Icon      *ui.SVG
	Style     ButtonStyle
	Size      ButtonSize
	IconSize  float32
	IconColor *ui.Color
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
		iconColor = semantic.Icon.Accent
	} else if b.IconColor != nil {
		iconColor = *b.IconColor
	}
	return buttonElement(c, semantic, buttonOptions{
		key:      b.Key,
		label:    b.Label,
		tooltip:  b.Tooltip,
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
	if options.selected && options.style != SubtleButton {
		border = semantic.Border.Selected
	} else if options.selected {
		border = semantic.Border.Variant
	}

	button := ui.ButtonBase(c.Key(options.key)).Label(options.label).
		Height(height).Padding(0).Radius(theme.DefaultControls.Scaled(scale).Radius).
		Border(1, border).Disabled(options.disabled)
	button.Tooltip(CommandTooltip(options.label, ""))
	if options.tooltip != "" {
		button.Tooltip(options.tooltip)
	}
	if options.disabled {
		button.Opacity(DisabledOpacity)
	}
	hoverAmount := HoverAmount(button, !options.disabled && button.Hovered(), options.animate)
	background := blend(rest, hover, hoverAmount)
	if !options.disabled && button.Pressed() {
		background = blend(background, semantic.Text.Default, 0.10)
	}
	button.Background(background)
	button.Children(options.children)
	if !options.disabled && button.Clicked() && options.action != nil {
		options.action()
	}
	return button
}

// CommandTooltip names a control and, when present, its live shortcut.
func CommandTooltip(label, binding string) string {
	if binding == "" {
		return label
	}
	return label + "  " + binding
}

func buttonColors(
	semantic theme.Theme,
	style ButtonStyle,
	selected bool,
) (rest, hover, border ui.Color) {
	switch style {
	case PrimaryButton:
		return semantic.Accent.Foreground, semantic.Accent.Foreground, ui.Transparent
	case DestructiveButton:
		return semantic.Status.Danger.Foreground, semantic.Status.Danger.Foreground, ui.Transparent
	case SubtleButton:
		rest = semantic.GhostElement.Background
		if selected {
			rest = semantic.Surface.Page
		}
		return rest, semantic.Surface.Page, ui.Transparent
	case OutlinedButton:
		return semantic.GhostElement.Background, semantic.GhostElement.Hover, semantic.Border.Variant
	case WarningButton:
		return semantic.GhostElement.Background, semantic.Status.Warning.Background, ui.Transparent
	default:
		return semantic.Element.Background, semantic.Element.Hover, semantic.Border.Default
	}
}

func buttonTextColor(semantic theme.Theme, style ButtonStyle) ui.Color {
	if style == PrimaryButton || style == DestructiveButton {
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

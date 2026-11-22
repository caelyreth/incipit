package components

import (
	"math"

	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type TextField struct {
	Key            string
	Label          string
	Placeholder    string
	Value          *string
	Icon           *ui.SVG
	ClearIcon      *ui.SVG
	Animate        bool
	AutoFocus      bool
	FocusRequested bool
	Submit         func()
	Changed        func()
	// Prominent is the field a surface is for, as the command palette's
	// query: a list row's height and the default text.
	Prominent bool
}

func (f TextField) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	height, text, icon := metrics.Height, theme.TextSmall, 16*scale/theme.DefaultUIFontSize
	if f.Prominent {
		height, text, icon = metrics.Row, theme.TextDefault, metrics.Icon
	}
	var input ui.Element
	field := ui.Row(c).Height(height).MinWidth(0).
		PaddingStart(theme.Space08.DIPs(scale)).
		PaddingEnd(theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Cursor(ui.CursorText)
	field.Children(func() {
		// A field without an icon starts its text at its edge.
		if f.Icon != nil {
			ui.Icon(c, f.Icon).FontSize(icon).
				TextColor(semantic.Icon.Muted)
		}
		input = ui.TextInputBase(c.Key(f.Key), f.Value).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(text.Size(scale)).
			FixedLineHeight(text.LineHeight(scale)).
			TextColor(semantic.Text.Default).
			Placeholder(f.Placeholder).Label(f.Label)
		if f.FocusRequested {
			input.Focus()
		}
		if input.Changed() && f.Changed != nil {
			f.Changed()
		}
		if input.Submitted() && f.Submit != nil {
			f.Submit()
		}
		if f.Value != nil && *f.Value != "" && f.ClearIcon != nil {
			IconButton{
				Key: "clear", Label: locales.Clear(), Icon: f.ClearIcon,
				Style: SubtleButton, Size: CompactButton,
				IconColor: semantic.Icon.Muted, Animate: f.Animate,
				Action: func() {
					input.ReplaceText(0, math.MaxInt, "").Focus(ui.FocusOptions{PreventScroll: true})
				},
			}.View(c, semantic)
		}
	})
	if f.AutoFocus {
		input.AutoFocus()
	}
	if input.Focused() {
		field.Border(1, semantic.Border.Focused)
	}
	return field
}

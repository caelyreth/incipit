package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type TextField struct {
	Key         string
	Label       string
	Placeholder string
	Value       *string
	Icon        *ui.SVG
	ClearIcon   *ui.SVG
	Animate     bool
	AutoFocus   bool
	Submit      func()
	Changed     func()
}

func (f TextField) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	var input ui.Element
	field := ui.Row(c).Height(metrics.Height).MinWidth(0).
		PaddingStart(theme.Space08.DIPs(scale)).
		PaddingEnd(theme.Space04.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Cursor(ui.CursorText)
	field.Children(func() {
		ui.Icon(c, f.Icon).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(semantic.Icon.Muted)
		input = ui.TextInputBase(c.Key(f.Key), f.Value).
			Grow(1).MinWidth(0).Font(theme.UIFontFamily).
			FontSize(theme.TextSmall.Size(scale)).
			FixedLineHeight(theme.TextSmall.LineHeight(scale)).
			TextColor(semantic.Text.Default).
			Placeholder(f.Placeholder).Label(f.Label)
		if input.Changed() && f.Changed != nil {
			f.Changed()
		}
		if input.Submitted() && f.Submit != nil {
			f.Submit()
		}
		if f.Value != nil && *f.Value != "" && f.ClearIcon != nil {
			IconButton{
				Key: "clear", Label: "Clear", Icon: f.ClearIcon,
				Style: SubtleButton, Size: CompactButton,
				IconColor: &semantic.Icon.Muted, Animate: f.Animate,
				Action: func() {
					*f.Value = ""
					input.SetChanged()
					input.Focus()
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

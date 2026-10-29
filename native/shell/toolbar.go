package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) formatBar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	scale := s.Settings.UIFontSize
	animate := s.animate(c)
	Icon := func(
		key, label, binding string,
		icon components.IconName,
		selected, enabled bool,
		action func(),
	) func() {
		return func() {
			components.IconButton{
				Key: key, Label: label, Tooltip: components.CommandTooltip(label, binding),
				Icon:  icon.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				Selected: selected, Disabled: !enabled, Animate: animate,
				IconColor: &semantic.Icon.Muted, Action: action,
			}.View(c, semantic)
		}
	}
	dock := components.ActionDock{
		Key: "writing-actions", Label: "Writing actions",
		Groups: []components.ActionGroup{
			{
				Label: "Formatting",
				Actions: []func(){
					Icon("format-emphasis", "Italic", s.Toolbar.Bindings.Italic, components.IconItalic,
						s.Toolbar.Italic, s.Toolbar.ItalicEnabled,
						func() {
							if s.FormatItalic != nil {
								s.FormatItalic(!s.Toolbar.Italic)
							}
						}),
					Icon("format-strong", "Bold", s.Toolbar.Bindings.Bold, components.IconBold,
						s.Toolbar.Bold, s.Toolbar.BoldEnabled,
						func() {
							if s.FormatBold != nil {
								s.FormatBold(!s.Toolbar.Bold)
							}
						}),
				},
			},
			{
				Label: "History",
				Actions: []func(){
					Icon("undo", "Undo", s.Toolbar.Bindings.Undo, components.IconUndo, false, s.CanUndo,
						func() {
							if s.Undo != nil {
								s.Undo()
							}
						}),
					Icon("redo", "Redo", s.Toolbar.Bindings.Redo, components.IconRedo, false, s.CanRedo,
						func() {
							if s.Redo != nil {
								s.Redo()
							}
						}),
				},
			},
			{
				Label: "Presentation",
				Actions: []func(){
					Icon("typewriter", "Typewriter", "", components.IconTypewriter,
						s.Appearance.Typewriter, true,
						func() {
							if s.Typewriter != nil {
								s.Typewriter(!s.Appearance.Typewriter)
							}
						}),
					Icon("zen", "Zen", "", components.IconZen, s.Appearance.Zen, true,
						func() {
							if s.Zen != nil {
								s.Zen(!s.Appearance.Zen)
							}
						}),
					Icon("cjk", "CJK", "", components.IconLanguages, s.Toolbar.CJK, true,
						func() {
							if s.ToggleCJK != nil {
								s.ToggleCJK(!s.Toolbar.CJK)
							}
						}),
				},
			},
		},
	}
	ui.Row(c).Absolute().Left(0).Right(0).
		Bottom(theme.DefaultLayout.DockOffset * scale / theme.DefaultUIFontSize).
		PaddingX(theme.Space16.DIPs(scale)).
		Justify(ui.Center).PassThrough().Children(func() {
		dock.View(c, semantic)
	})
	_ = metrics
}

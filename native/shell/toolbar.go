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
				Label: "Presentation",
				Actions: []func(){
					Icon("typewriter", "Typewriter", s.Toolbar.Bindings.Typewriter, components.IconTypewriter,
						s.Toolbar.Typewriter, s.Toolbar.TypewriterEnabled,
						func() {
							if s.Typewriter != nil {
								s.Typewriter(!s.Toolbar.Typewriter)
							}
						}),
					Icon("zen", "Zen", s.Toolbar.Bindings.Zen, components.IconZen,
						s.Toolbar.Zen, s.Toolbar.ZenEnabled,
						func() {
							if s.Zen != nil {
								s.Zen(!s.Toolbar.Zen)
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

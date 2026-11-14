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
					Icon("strong", "Strong", s.Toolbar.Bindings.Strong, components.IconBold,
						s.Toolbar.Formatting.Strong.On, s.Toolbar.Formatting.Strong.Enabled,
						func() {
							if s.Strong != nil {
								s.Strong()
							}
						}),
					Icon("emphasis", "Emphasis", s.Toolbar.Bindings.Emphasis, components.IconItalic,
						s.Toolbar.Formatting.Emphasis.On, s.Toolbar.Formatting.Emphasis.Enabled,
						func() {
							if s.Emphasis != nil {
								s.Emphasis()
							}
						}),
					Icon("code", "Code", s.Toolbar.Bindings.Code, components.IconCode,
						s.Toolbar.Formatting.Code.On, s.Toolbar.Formatting.Code.Enabled,
						func() {
							if s.Code != nil {
								s.Code()
							}
						}),
					Icon("strike", "Strike", s.Toolbar.Bindings.Strike, components.IconStrike,
						s.Toolbar.Formatting.Strike.On, s.Toolbar.Formatting.Strike.Enabled,
						func() {
							if s.Strike != nil {
								s.Strike()
							}
						}),
				},
			},
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
	dock.View(c, semantic).Transition(popTransition(animate))
	_ = metrics
}

// dock is what floats over the page's foot: the writing actions, or a
// sheet's find in their place while it is open.
func (s *State) dock(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	scale := s.Settings.UIFontSize
	animate := s.animate(c)
	ui.Row(c).Key("dock").Absolute().Left(0).Right(0).
		Bottom(theme.DefaultLayout.DockOffset * scale / theme.DefaultUIFontSize).
		PaddingX(theme.Space16.DIPs(scale)).
		Justify(ui.Center).PassThrough().Children(func() {
		if s.finder.open {
			s.findBarView(c, semantic, animate)
			return
		}
		s.formatBar(c, semantic, metrics)
	})
}

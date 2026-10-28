package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Divider(c).BorderColor(semantic.Border.Default)
	ui.Row(c).Height(layout.Status).Padding(0, theme.Space12.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Gap(theme.Space12.DIPs(s.Settings.UIFontSize)).Children(func() {
		ui.Text(c, s.Status).TextColor(semantic.Text.Muted)
		ui.Spacer(c)
		if s.Statistics != "" {
			ui.Text(c, s.Statistics).TextColor(semantic.Text.Muted)
		}
		if s.Appearance.Typewriter {
			ui.Text(c, "Typewriter").TextColor(semantic.Text.Muted)
		}
	})
}

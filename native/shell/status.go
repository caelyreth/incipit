package shell

import (
	"strings"

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
		AlignItems(ui.Center).Gap(theme.Space16.DIPs(s.Settings.UIFontSize)).Children(func() {
		statistics := s.Statistics
		if statistics == "" {
			statistics = "No open document"
		}
		ui.Text(c, statistics).Grow(1).MinWidth(0).SingleLine().
			FontSize(theme.TextXSmall.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Muted)
		status := s.Status
		if status == "" {
			status = s.modeStatus()
		}
		if status != "" {
			ui.Text(c, status).MaxWidthPercent(45).SingleLine().TextAlign(ui.End).
				FontSize(theme.TextXSmall.Size(s.Settings.UIFontSize)).
				TextColor(semantic.Text.Muted)
		}
	})
}

func (s *State) modeStatus() string {
	parts := make([]string, 0, 2)
	if s.Appearance.Typewriter {
		parts = append(parts, "Typewriter")
	}
	if s.Toolbar.CJK {
		parts = append(parts, "CJK refinement")
	}
	return strings.Join(parts, "  ·  ")
}

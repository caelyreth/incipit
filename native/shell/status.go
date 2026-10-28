package shell

import (
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
	chrome float32,
) {
	scale := s.Settings.UIFontSize
	height := layout.Status * scale / theme.DefaultUIFontSize * chrome
	statistics := s.Statistics
	if statistics == "" {
		statistics = "No open document"
	}
	status := s.Status
	if status == "" {
		status = s.modeStatus()
	}
	ui.Row(c).Height(height).Opacity(chrome).Clip().
		PaddingX(layout.ContentGutter*scale/theme.DefaultUIFontSize).
		AlignItems(ui.Center).Gap(theme.Space16.DIPs(scale)).
		BorderWidth(1, 0, 0, 0).BorderColor(semantic.Border.Variant).
		Background(semantic.Surface.Background).Children(func() {
		components.Label(c, statistics, theme.TextXSmall, semantic.Text.Muted).
			Grow(1).MinWidth(0).SingleLine().FontFeatures("tnum")
		if status != "" {
			components.Label(c, status, theme.TextXSmall, semantic.Text.Muted).
				MaxWidthPercent(45).SingleLine().TextAlign(ui.End).FontFeatures("tnum")
		}
	})
	_ = metrics
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

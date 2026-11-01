package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// footer counts the open page's words, paragraphs and characters, at the
// end of the line below it.
func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	layout theme.Layout,
	chrome float32,
) {
	scale := s.Settings.UIFontSize
	height := layout.Status * scale / theme.DefaultUIFontSize * chrome
	statistics := s.Statistics
	if statistics == "" {
		statistics = "No open document"
	}
	ui.Row(c).Height(height).Opacity(chrome).Clip().
		PaddingX(layout.ContentGutter * scale / theme.DefaultUIFontSize).
		AlignItems(ui.Center).Justify(ui.End).
		Background(semantic.Surface.Page).Children(func() {
		components.Label(c, statistics, theme.TextXSmall, semantic.Text.Muted).
			MinWidth(0).SingleLine().FontFeatures("tnum")
	})
}

package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// footer shows the open page's counts, as Statistics spells them, at the
// end of the line below it; it is an empty line while there are none.
func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	layout theme.Layout,
	chrome float32,
) {
	scale := s.Settings.UIFontSize
	height := layout.Status * scale / theme.DefaultUIFontSize * chrome
	ui.Row(c).Height(height).Opacity(chrome).Clip().
		PaddingX(layout.ContentGutter * scale / theme.DefaultUIFontSize).
		AlignItems(ui.Center).Justify(ui.End).
		Background(semantic.Surface.Page).Children(func() {
		if s.Statistics != "" {
			components.Label(c, s.Statistics, theme.TextXSmall, semantic.Text.Muted).
				MinWidth(0).SingleLine().FontFeatures("tnum")
		}
	})
}

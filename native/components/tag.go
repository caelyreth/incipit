package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// TagChip is a tag as the app shows one beside the page, in the sheet
// list and the page's tag picker: its text at size on fill, rounded as the
// page rounds a tag's background, cut short where width cannot hold it.
func TagChip(c *ui.Context, text string, size theme.TextSize, pad, width float32, fill, ink ui.Color) ui.Element {
	scale := UIFontSize(c)
	chip := ui.Row(c).Height(size.LineHeight(scale) + theme.Space04.DIPs(scale)).
		Radius(3 * scale / theme.DefaultUIFontSize).PaddingX(pad).
		MaxWidth(width).MinWidth(0).AlignItems(ui.Center).Background(fill)
	chip.Children(func() {
		CenteredLabel(c, text, size, ink).MinWidth(0)
	})
	return chip
}

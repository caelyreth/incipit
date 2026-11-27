package components

import (
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Wordmark is Incipit's name as the app sets it wherever it names
// itself, size DIPs: the UI face, bold and drawn a little tight, in the
// text's ink, its full stop in the accent's, the two on one baseline.
// The field's horizon and the onboarding's cover show this one drawing.
func Wordmark(c *ui.Context, key string, semantic theme.Theme, size float32) ui.Element {
	return ui.Row(c).Key(key).AlignItems(ui.End).Children(func() {
		ui.Text(c, "Incipit").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
			LetterSpacing(-0.02 * size).TextColor(semantic.Text.Default).SingleLine()
		ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
			TextColor(semantic.Accent.Foreground).SingleLine()
	})
}

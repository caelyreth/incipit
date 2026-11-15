package components

import (
	"math"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// WithTextOrigin draws text from a fixed horizontal device-pixel phase,
// carrying its fractional position in one translation. A shaped run, or
// a formula's glyphs and rules, then moves as a whole. The supplied
// origin and translation add up to at; logical geometry stays unchanged.
func WithTextOrigin(p *ui.Painter, at ui.Point, draw func(ui.Point)) {
	origin := at
	origin.X = float32(math.Round(float64(at.X*p.Scale()))) / p.Scale()
	p.Transform(ui.Translation(at.X-origin.X, 0), 0, 0, func() { draw(origin) })
}

// UIFontSize returns the window's current rem size in DIPs.
func UIFontSize(c *ui.Context) float32 {
	if c == nil || c.Theme() == nil || c.Theme().FontSize <= 0 {
		return theme.DefaultUIFontSize
	}
	return c.Theme().FontSize
}

// Label applies Neverland's chrome type scale, including its paired line
// height. Raw MyGo text uses a different default leading.
func Label(
	c *ui.Context,
	text string,
	size theme.TextSize,
	color ui.Color,
) ui.Element {
	scale := UIFontSize(c)
	return ui.Text(c, text).
		Font(theme.UIFontFamily).
		FontSize(size.Size(scale)).
		FixedLineHeight(size.LineHeight(scale)).
		TextColor(color)
}

// Headline applies the settings-page heading scale and alpha=1.4 leading.
func Headline(
	c *ui.Context,
	text string,
	size theme.HeadlineSize,
	color ui.Color,
) ui.Element {
	scale := UIFontSize(c)
	return ui.Text(c, text).
		Font(theme.UIFontFamily).
		FontSize(size.Size(scale)).
		LineHeight(theme.HeadlineLineHeight).
		TextColor(color)
}

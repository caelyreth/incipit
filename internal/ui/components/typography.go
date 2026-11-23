package components

import (
	"math"

	"github.com/caelyreth/neverland/internal/ui/theme"
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

// UIOpticalLift returns how far single-line UI-face chrome text rises to center
// optically inside a line box or control: the UI face's ink sits slightly below
// its line box center.
func UIOpticalLift(scale float32) float32 {
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	return 0.5 * scale / theme.DefaultUIFontSize
}

// InlineCapMiddle returns how far above a sentence's text baseline the optical
// middle of inline UI-face cap/lowercase ink sits.
func InlineCapMiddle(fontSize float32) float32 {
	return 0.33 * fontSize
}

// GlyphCenterBaseline returns the drawing baseline Y that centers UI-face
// glyphs of size glyphSize vertically around centerY.
func GlyphCenterBaseline(centerY, glyphSize float32) float32 {
	return centerY + 0.36*glyphSize
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

// CenteredLabel returns a single-line UI-face label lifted optically so its
// glyphs center vertically inside buttons, chips, tabs, and rows.
func CenteredLabel(
	c *ui.Context,
	text string,
	size theme.TextSize,
	color ui.Color,
) ui.Element {
	scale := UIFontSize(c)
	return Label(c, text, size, color).SingleLine().Translate(0, -UIOpticalLift(scale))
}

// BaselineRow returns a horizontal row that aligns mixed-size chrome labels on
// a shared baseline and lifts them optically as a group.
func BaselineRow(c *ui.Context) ui.Element {
	return ui.Row(c).AlignItems(ui.Baseline).Translate(0, -UIOpticalLift(UIFontSize(c)))
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

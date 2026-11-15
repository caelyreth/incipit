package theme

import "math"

const (
	ProseFontFamily    = "Noto Serif CJK SC"
	UIFontFamily       = "system-ui"
	MonoFontFamily     = "monospace"
	referenceFontSize  = float32(19)
	referenceCaret     = float32(25)
	referenceCaretDrop = float32(1)
	referenceColumn    = float32(700)
	MinProseFontSize   = float32(15)
	MaxProseFontSize   = float32(26)
)

// ProseStyle is document text's size and its line spacing: a body line's
// height over its text's size.
type ProseStyle struct {
	FontSize float32
	Spacing  float32
}

const (
	MinLineSpacing     = float32(1.5)
	MaxLineSpacing     = float32(2.2)
	DefaultLineSpacing = float32(1.8)
)

func NewProseStyle(fontSize float32) ProseStyle {
	return ProseStyle{
		FontSize: clamp(fontSize, MinProseFontSize, MaxProseFontSize),
		Spacing:  DefaultLineSpacing,
	}
}

// WithSpacing is the style at another line spacing.
func (p ProseStyle) WithSpacing(spacing float32) ProseStyle {
	p.Spacing = clamp(spacing, MinLineSpacing, MaxLineSpacing)
	return p
}

func (p ProseStyle) lineHeight() float32 {
	return float32(math.Round(float64(p.FontSize * p.Spacing)))
}

// Leading is the room a body line leaves beside its text, kept between
// every line whatever the size of its text.
func (p ProseStyle) Leading() float32 {
	return p.lineHeight() - p.FontSize
}

func (p ProseStyle) CaretHeight() float32 {
	return p.scaled(referenceCaret)
}

// CaretOffsetY is a small optical drop from the line's geometric center.
func (p ProseStyle) CaretOffsetY() float32 {
	return referenceCaretDrop * p.FontSize / referenceFontSize
}

func (p ProseStyle) Column() float32 {
	return p.scaled(referenceColumn)
}

// Measure is the text width in a viewport, leaving a body em at either
// side until the reading column fits.
func (p ProseStyle) Measure(viewport float32) float32 {
	return min(p.Column(), max(viewport-2*p.FontSize, p.FontSize*8))
}

// Inset centers the text measure in a viewport.
func (p ProseStyle) Inset(viewport float32) float32 {
	return max((viewport-p.Measure(viewport))/2, 0)
}

func (p ProseStyle) LineHeight() float32 {
	return p.lineHeight()
}

func (p ProseStyle) scaled(reference float32) float32 {
	return float32(math.Round(float64(reference * p.FontSize / referenceFontSize)))
}

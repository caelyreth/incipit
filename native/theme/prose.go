package theme

import "math"

const (
	ProseFontFamily    = "Noto Serif CJK SC"
	UIFontFamily       = "system-ui"
	referenceFontSize  = float32(19)
	referenceLine      = float32(34)
	referenceCaret     = float32(25)
	referenceCaretDrop = float32(1)
	referenceTitle     = float32(34)
	referenceTitleLine = float32(48)
	referenceColumn    = float32(700)
	MinProseFontSize   = float32(15)
	MaxProseFontSize   = float32(26)
)

type ProseStyle struct {
	FontSize float32
}

func NewProseStyle(fontSize float32) ProseStyle {
	return ProseStyle{FontSize: clamp(fontSize, MinProseFontSize, MaxProseFontSize)}
}

func (p ProseStyle) lineHeight() float32 {
	return p.scaled(referenceLine)
}

func (p ProseStyle) CaretHeight() float32 {
	return p.scaled(referenceCaret)
}

// CaretOffsetY is a small optical drop from the line's geometric center.
func (p ProseStyle) CaretOffsetY() float32 {
	return referenceCaretDrop * p.FontSize / referenceFontSize
}

func (p ProseStyle) TitleSize() float32 {
	return p.scaled(referenceTitle)
}

func (p ProseStyle) TitleLineHeight() float32 {
	return p.scaled(referenceTitleLine)
}

func (p ProseStyle) Column() float32 {
	return p.scaled(referenceColumn)
}

func (p ProseStyle) LineHeight() float32 {
	return p.lineHeight()
}

func (p ProseStyle) scaled(reference float32) float32 {
	return float32(math.Round(float64(reference * p.FontSize / referenceFontSize)))
}

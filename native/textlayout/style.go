// Package textlayout retains MyGo shaped paragraphs and maps them to the
// engine's UTF-16 coordinates.
package textlayout

import (
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type Style struct {
	Prose theme.ProseStyle
	Theme theme.Theme
}

func NewStyle(prose theme.ProseStyle, semantic theme.Theme) Style {
	return Style{Prose: prose, Theme: semantic}
}

func (s Style) font() ui.Font {
	return ui.Font{
		Family:     theme.ProseFontFamily,
		Size:       s.Prose.FontSize,
		LineHeight: s.Prose.LineHeight() / s.Prose.FontSize,
	}
}

func (s Style) color(role surface.ColorRole) ui.Color {
	switch role {
	case surface.ColorMuted:
		return s.Theme.Prose.Notation
	case surface.ColorHeading:
		return s.Theme.Prose.Heading
	case surface.ColorAccent:
		return s.Theme.Accent.Foreground
	case surface.ColorSubtle:
		return s.Theme.Prose.Rule
	default:
		return s.Theme.Prose.Text
	}
}

func (s Style) span(text string, run surface.StyleRun) ui.Span {
	weight := 400
	if run.Style.Bold {
		weight = 700
	}
	family, size := theme.ProseFontFamily, s.Prose.FontSize*headingScale(run.Style.Heading)
	if run.Style.Code {
		family, size = "monospace", size*codeScale
	}
	return ui.Span{
		Text:          text,
		Font:          family,
		Size:          size,
		Weight:        weight,
		Italic:        run.Style.Italic,
		Strikethrough: run.Style.Strike,
		Color:         s.color(run.Style.Color),
	}
}

// codeScale sets code a little smaller than the prose around it, so the
// monospace face's larger x-height reads at the same size.
const codeScale = 0.88

// headingScale is how much larger than body text a heading of a level is
// set: a step down per level, the last two at body size.
func headingScale(level uint8) float32 {
	switch level {
	case 1:
		return 1.6
	case 2:
		return 1.36
	case 3:
		return 1.18
	case 4:
		return 1.06
	}
	return 1
}

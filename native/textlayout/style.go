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
	return ui.Span{
		Text:   text,
		Font:   theme.ProseFontFamily,
		Size:   s.Prose.FontSize,
		Weight: weight,
		Italic: run.Style.Italic,
		Color:  s.color(run.Style.Color),
	}
}

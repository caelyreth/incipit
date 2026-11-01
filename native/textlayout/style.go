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
	// Spread is how far past the column each side a block wider than it,
	// as a table, may reach into the margins.
	Spread float32
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
	case surface.ColorStrong:
		return s.Theme.Prose.Strong
	case surface.ColorAccent:
		return s.Theme.Accent.Foreground
	case surface.ColorSubtle:
		return s.Theme.Prose.Subtle
	default:
		return s.Theme.Prose.Text
	}
}

func (s Style) span(text string, run surface.StyleRun) ui.Span {
	weight := 400
	if run.Style.Bold {
		weight = 700
	}
	family := theme.ProseFontFamily
	if run.Style.Face == surface.FaceMono {
		family = theme.MonoFontFamily
	}
	return ui.Span{
		Text:          text,
		Font:          family,
		Size:          s.Prose.FontSize * run.Style.Size(),
		Weight:        weight,
		Italic:        run.Style.Italic,
		Strikethrough: run.Style.Strike,
		Color:         s.color(run.Style.Color),
	}
}

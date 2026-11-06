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

// RoleColor resolves a surface color role in the native theme. Text shaping
// and document decorations use the same mapping.
func RoleColor(semantic theme.Theme, role surface.ColorRole) ui.Color {
	switch role {
	case surface.ColorMuted:
		return semantic.Prose.Notation
	case surface.ColorStrong:
		return semantic.Prose.Strong
	case surface.ColorAccent:
		return semantic.Accent.Foreground
	case surface.ColorSubtle:
		return semantic.Prose.Subtle
	default:
		return semantic.Prose.Text
	}
}

// The page sets each size its text's role says: titles a step down per
// level, a caption a step below the text, and code a little smaller than
// the text it stands in, so the mono face's larger x-height reads at the
// same size.
const (
	// codeScale is mono text's size over the size its role says.
	codeScale = 0.88
	// captionScale is a caption's size over the page's text.
	captionScale = 0.88
)

// titleScales are each title's size over the page's text, the first's
// first.
var titleScales = [...]float32{1.6, 1.36, 1.18, 1.06}

// textSize is the size the page sets text of style at.
func (s Style) textSize(style surface.TextStyle) float32 {
	size := s.Prose.FontSize
	switch role := style.Size; {
	case role >= surface.SizeTitle1 && role <= surface.SizeTitle4:
		size *= titleScales[role-surface.SizeTitle1]
	case role == surface.SizeCaption:
		size *= captionScale
	}
	if style.Face == surface.FaceMono {
		size *= codeScale
	}
	return size
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
		Size:          s.textSize(run.Style),
		Weight:        weight,
		Italic:        run.Style.Italic,
		Strikethrough: run.Style.Strike,
		Color:         RoleColor(s.Theme, run.Style.Color),
	}
}

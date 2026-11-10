// Package page is the native renderer of a page: it lays out the units the
// engine presents, paints them with their decorations, and turns pointer,
// key and text-input events into intents. It holds no document state of its
// own beyond geometry; everything it shows comes from the engine.
package page

import (
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// headingScale is each heading level's size over the body's.
var headingScale = [7]float32{1, 1.75, 1.45, 1.25, 1.12, 1.05, 1}

// Theme maps the vocabulary's highlight groups to styles in the product's
// semantic colors. Policy groups fall back to these through their chains.
func Theme(t theme.Theme) present.Theme {
	th := present.Theme{
		vocab.GroupText:     {Set: present.SetFont | present.SetColor, Font: present.FontBody, Color: pack(t.Prose.Text)},
		vocab.GroupHeading:  {Set: present.SetFont | present.SetWeight | present.SetColor, Font: present.FontHeading, Weight: 700, Color: pack(t.Prose.Strong)},
		vocab.GroupEmphasis: tinted(t.Families[theme.Unripe]),
		vocab.GroupStrong:   bold(tinted(t.Families[theme.Regolith])),
		vocab.GroupCodeMark: {Set: present.SetFont | present.SetBackground | present.SetSize, Font: present.FontMono,
			Size: 0.9, Background: pack(t.Surface.Inset)},
		vocab.GroupStrike:        {Set: present.SetStrike | present.SetColor, Strike: true, Color: pack(t.Text.Muted)},
		vocab.GroupGhost:         {Set: present.SetColor, Color: pack(t.Prose.Notation)},
		vocab.GroupCode:          {Set: present.SetFont | present.SetSize, Font: present.FontMono, Size: 0.9},
		vocab.GroupFormulaSource: {Set: present.SetFont | present.SetColor, Font: present.FontMono, Color: pack(t.Accent.Foreground)},
		vocab.GroupQuote:         {Set: present.SetColor, Color: pack(t.Text.Muted)},
		vocab.GroupListMarker:    {Set: present.SetColor, Color: pack(t.Prose.Notation)},
		vocab.GroupCaption:       {Set: present.SetColor | present.SetSize, Color: pack(t.Text.Muted), Size: 0.9},
		vocab.GroupCellHeader:    {Set: present.SetWeight, Weight: 700},
		vocab.GroupComposition:   {Set: present.SetUnderline, Underline: true},
	}
	for level := 1; level <= 6; level++ {
		th[vocab.GroupHeadings[level]] = present.Style{Set: present.SetSize, Size: headingScale[level]}
	}
	return th
}

// tinted is a mark worn in a hue: its family's foreground on its surface.
func tinted(f theme.Family) present.Style {
	return present.Style{Set: present.SetColor | present.SetBackground, Color: pack(f.Foreground), Background: pack(f.Surface)}
}

func bold(s present.Style) present.Style {
	s.Set |= present.SetWeight
	s.Weight = 700
	return s
}

// family is a font role's family.
func family(r present.FontRole) string {
	switch r {
	case present.FontMono, present.FontMath:
		// TODO(formulas): closed formulas typeset in the math font through
		// native/formula once the formula policy presents them as math.
		return theme.MonoFontFamily
	}
	return theme.ProseFontFamily
}

func pack(c ui.Color) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

func unpack(v uint32) ui.Color {
	return ui.RGBA(uint8(v>>24), uint8(v>>16), uint8(v>>8), float32(uint8(v))/255)
}

// span is a style's span of text at the body size.
func span(text string, s present.Style, body float32) ui.Span {
	sp := ui.Span{
		Text:          text,
		Font:          family(s.Font),
		Size:          body * s.Size,
		Weight:        s.Weight,
		Italic:        s.Italic,
		Underline:     s.Underline,
		Strikethrough: s.Strike,
	}
	if s.Set&present.SetColor != 0 {
		c := unpack(s.Color)
		if s.Opacity < 1 {
			c = c.Alpha(float32(c.A) / 255 * s.Opacity)
		}
		sp.Color = c
	}
	if s.Set&present.SetBackground != 0 {
		sp.Background = unpack(s.Background)
	}
	return sp
}

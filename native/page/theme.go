// Package page is the native renderer of a page: it lays out the units the
// engine presents, paints them with their decorations, and turns pointer,
// key and text-input events into intents. It holds no document state of its
// own beyond geometry; everything it shows comes from the engine.
package page

import (
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
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
		vocab.GroupText: {Set: present.SetFont | present.SetColor, Font: present.FontBody, Color: pack(t.Prose.Text)},
		profile.GroupHeading: {Set: present.SetFont | present.SetWeight | present.SetColor | present.SetSpace, Font: present.FontHeading,
			Weight: 700, Color: pack(t.Prose.Strong), Space: 1.6},
		profile.GroupEmphasis: tinted(t.Families[theme.Unripe]),
		profile.GroupStrong:   bold(tinted(t.Families[theme.Regolith])),
		profile.GroupCodeMark: {Set: present.SetFont | present.SetBackground | present.SetSize, Font: present.FontMono,
			Size: 0.9, Background: pack(t.Surface.Inset)},
		profile.GroupStrike:        {Set: present.SetStrike | present.SetColor, Strike: true, Color: pack(t.Text.Muted)},
		vocab.GroupGhost:           {Set: present.SetColor, Color: pack(t.Prose.Notation)},
		profile.GroupCode:          {Set: present.SetFont | present.SetSize, Font: present.FontMono, Size: 0.9},
		profile.GroupFormulaSource: {Set: present.SetFont | present.SetColor, Font: present.FontMono, Color: pack(t.Accent.Foreground)},
		profile.GroupQuote:         {Set: present.SetColor, Color: pack(t.Text.Muted)},
		profile.GroupListMarker:    {Set: present.SetColor, Color: pack(t.Prose.Notation)},
		// A caption is set smaller than the text, so it reads as the
		// picture's and not as a paragraph's.
		profile.GroupCaption: {Set: present.SetColor | present.SetSize, Color: pack(t.Text.Muted), Size: 0.8},
		// A table stands apart from the text around it: further than a
		// figure, short of a line, so an empty line by it still reads as
		// one. Its header is set as the app's own headings are.
		profile.GroupTable:      {Set: present.SetSpace | present.SetAfter, Space: 1.75, After: 1.75},
		profile.GroupCellHeader: {Set: present.SetWeight, Weight: 600},
		vocab.GroupComposition: {Set: present.SetLine | present.SetLineColor, Line: present.LineUnder,
			LineColor: pack(t.Families[theme.Halo].Foreground)},
		// Every paint has a look on text and around an object it holds
		// whole: the selection washes text and outlines an object in the
		// accent; what the next delete takes is outlined in the danger hue
		// wherever it is, text or object, so one look means "goes next".
		vocab.GroupSelection: {Set: present.SetBackground | present.SetOutline, Background: pack(t.Prose.Selection),
			Outline: pack(t.Accent.Foreground.Alpha(0.85))},
		vocab.GroupArmed: {Set: present.SetLine | present.SetLineColor | present.SetOutline,
			Line: present.LineBox, LineColor: pack(t.Families[theme.Remnant].Foreground), Outline: pack(t.Families[theme.Remnant].Foreground)},
		// A tag reads as one, in Halo whatever the accent: its surface,
		// the family's dimmest tone, behind its normal one, as the sheet
		// list shows tags.
		profile.GroupTag: {Set: present.SetBackground | present.SetColor,
			Background: pack(t.Families[theme.Halo].Surface), Color: pack(t.Families[theme.Halo].Foreground)},
		vocab.GroupPairMatch: {Set: present.SetBackground, Background: pack(t.Families[t.AccentFamily].Surface)},
		vocab.GroupScope: {Set: present.SetLine | present.SetLineColor, Line: present.LineUnder,
			LineColor: pack(t.Families[t.AccentFamily].Muted)},
		vocab.GroupPulse: {Set: present.SetBackground | present.SetLine | present.SetLineColor | present.SetOutline,
			Background: pack(t.Families[theme.Life].Surface), Line: present.LineUnder,
			LineColor: pack(t.Families[theme.Life].Foreground), Outline: pack(t.Families[theme.Life].Foreground)},
	}
	for level := 1; level <= 6; level++ {
		th[profile.GroupHeadings[level]] = present.Style{Set: present.SetSize, Size: headingScale[level]}
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
		// A formula's source is mono; its math is set by native/formula
		// (formulas.go).
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
		Strikethrough: s.Strike,
	}
	if s.Set&present.SetColor != 0 {
		c := unpack(s.Color)
		if s.Opacity < 1 {
			c = c.Alpha(float32(c.A) / 255 * s.Opacity)
		}
		sp.Color = c
	}
	return sp
}

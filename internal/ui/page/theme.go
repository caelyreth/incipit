// Package page is the native renderer of a page: it lays out the units the
// engine presents, paints them with their decorations, and turns pointer,
// key and text-input events into intents. It holds no document state of its
// own beyond geometry; everything it shows comes from the engine.
package page

import (
	"cmp"

	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/theme"
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
		profile.GroupEmphasis: {Set: present.SetColor | present.SetBackground,
			Color: pack(t.Families[theme.Unripe].Foreground), Background: pack(t.Families[theme.Unripe].Surface.Alpha(0.55))},
		profile.GroupStrong: {Set: present.SetWeight | present.SetColor | present.SetBackground, Weight: 700,
			Color: pack(t.Families[theme.Regolith].Foreground), Background: pack(t.Families[theme.Regolith].Surface.Alpha(0.55))},
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
			Outline: pack(t.Accent.Strong)},
		vocab.GroupArmed: {Set: present.SetLine | present.SetLineColor | present.SetOutline,
			Line: present.LineBox, LineColor: pack(t.Families[theme.Remnant].Foreground), Outline: pack(t.Families[theme.Remnant].Foreground)},
		// A tag reads as one, quietly: the neutral fill a list shows tags
		// in, so tags add no hue to the page.
		profile.GroupTag: {Set: present.SetBackground | present.SetColor,
			Background: pack(t.Tag.Background), Color: pack(t.Tag.Foreground)},
		// A word the checker does not know waves under it in the danger
		// hue's emphasis, and grammar it questions in a calmer hue's, as
		// the system's do: the strongest of each, so the wave reads on the
		// selection's wash too.
		profile.GroupMisspelled: {Set: present.SetLine | present.SetLineColor, Line: present.LineWave,
			LineColor: pack(t.Families[theme.Remnant].Emphasis)},
		profile.GroupGrammar: {Set: present.SetLine | present.SetLineColor, Line: present.LineWave,
			LineColor: pack(t.Families[theme.Life].Emphasis)},
		profile.GroupCorrected: {Set: present.SetLine | present.SetLineColor, Line: present.LineUnder,
			LineColor: pack(t.Families[theme.Halo].Foreground)},
		vocab.GroupPairMatch: {Set: present.SetBackground, Background: pack(t.Families[t.AccentFamily].Surface.Alpha(0.60))},
		vocab.GroupScope: {Set: present.SetLine | present.SetLineColor, Line: present.LineUnder,
			LineColor: pack(t.Families[t.AccentFamily].Muted)},
		vocab.GroupPulse: {Set: present.SetBackground | present.SetLine | present.SetLineColor | present.SetOutline,
			Background: pack(t.Families[theme.Life].Surface.Alpha(0.65)), Line: present.LineUnder,
			LineColor: pack(t.Families[theme.Life].Foreground), Outline: pack(t.Families[theme.Life].Foreground)},
	}
	// A find's matches are underlined in Regolith, quietly; the one the
	// selection went to flashes in the hue and keeps the selection's wash,
	// so the current match is the loudest on the page.
	th[vocab.GroupFindMatch] = present.Style{Set: present.SetLine | present.SetLineColor, Line: present.LineUnder,
		LineColor: pack(t.Families[theme.Regolith].Foreground)}
	th[vocab.GroupFindFlash] = present.Style{Set: present.SetBackground | present.SetLine | present.SetLineColor | present.SetOutline,
		Background: pack(t.Families[theme.Regolith].Muted), Line: present.LineBox,
		LineColor: pack(t.Families[theme.Regolith].Foreground), Outline: pack(t.Families[theme.Regolith].Foreground)}
	for level := 1; level <= 6; level++ {
		th[profile.GroupHeadings[level]] = present.Style{Set: present.SetSize, Size: headingScale[level]}
	}
	return th
}

// family is a font role's family under m.
func family(r present.FontRole, m Metrics) string {
	switch r {
	case present.FontMono, present.FontMath:
		// A formula's source is mono; its math is set by internal/ui/formula
		// (formulas.go).
		return theme.MonoFontFamily
	}
	return cmp.Or(m.Family, theme.ProseFontFamily)
}

func pack(c ui.Color) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

func unpack(v uint32) ui.Color {
	return ui.RGBA(uint8(v>>24), uint8(v>>16), uint8(v>>8), float32(uint8(v))/255)
}

// span is a style's span of text under m, at its body size.
// Colors are resolved at paint time so theme transitions do not invalidate text shaping.
func span(text string, s present.Style, m Metrics) ui.Span {
	return ui.Span{
		Text:          text,
		Font:          family(s.Font, m),
		Size:          m.Body * s.Size,
		Weight:        s.Weight,
		Italic:        s.Italic,
		Strikethrough: s.Strike,
	}
}

func styleColor(s present.Style) ui.Color {
	if s.Set&present.SetColor == 0 {
		return ui.Color{}
	}
	c := unpack(s.Color)
	if s.Opacity < 1 {
		c = c.Alpha(float32(c.A) / 255 * s.Opacity)
	}
	return c
}

const layoutStyleMask = present.SetFont | present.SetSize | present.SetWeight | present.SetItalic |
	present.SetLine | present.SetStrike | present.SetSpace | present.SetAfter |
	present.SetBackground | present.SetOutline

func sameLayoutStyle(a, b present.Style) bool {
	if a.Set&layoutStyleMask != b.Set&layoutStyleMask {
		return false
	}
	return a.Font == b.Font && a.Size == b.Size && a.Weight == b.Weight && a.Italic == b.Italic &&
		a.Line == b.Line && a.Strike == b.Strike && a.Space == b.Space && a.After == b.After &&
		(a.Background != 0) == (b.Background != 0) && (a.Outline != 0) == (b.Outline != 0)
}

func sameLayoutTheme(a, b present.Theme) bool {
	if len(a) != len(b) {
		return false
	}
	for k, sa := range a {
		sb, ok := b[k]
		if !ok || !sameLayoutStyle(sa, sb) {
			return false
		}
	}
	return true
}

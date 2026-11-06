package editor

import (
	"strings"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// The kinds of inline mark. Each is described once, below, as block kinds
// are in kinds.go: the rest of the editor asks a mark's descriptor what it
// is, never its kind's name (kinds_test.go keeps it so).
const (
	// EmphasisMark is emphasis: level one for emphasis, two for strong.
	EmphasisMark model.MarkKind = "emphasis"
	// CodeMark is inline code.
	CodeMark model.MarkKind = "code"
	// StrikeMark is struck-through text.
	StrikeMark model.MarkKind = "strike"
	// MathMark is an inline formula; the text it covers is its TeX source.
	MathMark model.MarkKind = "math"
)

// FormatMark is the mark Format toggles, by level.
const FormatMark = EmphasisMark

// MarkDesc is what a kind of mark is, as the editor treats it.
type MarkDesc struct {
	Kind model.MarkKind
	// Glyphs are the markers that open and close it, each glyph once a
	// level: emphasis's "*" or "_".
	Glyphs string
	// Top is the highest level typing its markers reaches; Cycles takes a
	// marker typed past it back to none.
	Top    uint8
	Cycles bool
	// Literal text is taken as written: only its own marker, at its end,
	// is notation within it.
	Literal bool
	// Formula text is TeX: a place of its own, edited on its own card.
	Formula bool
	// Escapes is a backslash before a marker making it text, as TeX's \$.
	Escapes bool
	// Fences are markers all a paragraph holds, which may be opening a
	// fenced block rather than the mark.
	Fences bool
	// Opens is the block an empty mark closed at once, alone in an empty
	// paragraph, opens: "$$" a formula block.
	Opens model.NodeTypeID

	open, close projection.Role
	style       func(level uint8, style *surface.TextStyle)
	allowed     func(t writingTriggers, glyph rune) bool
}

var markTable = []MarkDesc{
	{
		Kind: EmphasisMark, Glyphs: "*_", Top: 2, Cycles: true,
		style: func(level uint8, style *surface.TextStyle) {
			switch level {
			case 1:
				style.Italic = !style.Italic
			case 2:
				style.Bold = true
			}
		},
		allowed: func(t writingTriggers, glyph rune) bool {
			return glyph == '*' && t.asterisks || glyph == '_' && t.underscores
		},
	},
	{
		Kind: CodeMark, Glyphs: "`", Top: 1, Literal: true, Fences: true,
		style:   func(_ uint8, style *surface.TextStyle) { codeStyle(style) },
		allowed: func(t writingTriggers, _ rune) bool { return t.code },
	},
	{
		Kind: MathMark, Glyphs: "$", Top: 1, Literal: true, Formula: true, Escapes: true, Opens: MathNode,
		open: roleFormulaOpen, close: roleFormulaClose,
		style:   func(_ uint8, style *surface.TextStyle) { codeStyle(style) },
		allowed: func(t writingTriggers, _ rune) bool { return t.math },
	},
	{
		Kind: StrikeMark, Glyphs: "~", Top: 1,
		style:   func(_ uint8, style *surface.TextStyle) { style.Strike = true },
		allowed: func(t writingTriggers, _ rune) bool { return t.strike },
	},
}

var unknownMark MarkDesc

// describeMark returns what kind of mark kind is.
func describeMark(kind model.MarkKind) *MarkDesc {
	for i := range markTable {
		if markTable[i].Kind == kind {
			return &markTable[i]
		}
	}
	return &unknownMark
}

// markOf is what mark is.
func markOf(mark model.InlineMark) *MarkDesc { return describeMark(mark.Kind()) }

// DescribeMark returns what kind of mark the editor takes kind for.
func DescribeMark(kind model.MarkKind) MarkDesc { return *describeMark(kind) }

// markerKind is the mark a typed marker opens, when typing it is allowed to,
// and the highest level the mark goes to.
func (t writingTriggers) markerKind(text string) (model.MarkKind, uint8, bool) {
	if len(text) != 1 {
		return "", 0, false
	}
	glyph := rune(text[0])
	for _, desc := range markTable {
		if strings.ContainsRune(desc.Glyphs, glyph) {
			return desc.Kind, desc.Top, desc.allowed(t, glyph)
		}
	}
	return "", 0, false
}

// inlineNotation is how a mark shows on either side of its text: its first
// marker, once a level.
func inlineNotation(mark model.InlineMark) string {
	desc := markOf(mark)
	if desc.Glyphs == "" {
		return ""
	}
	return strings.Repeat(desc.Glyphs[:1], int(mark.Level()))
}

// inlineLevel reads edited notation of a mark back to its level; zero takes
// the mark away. Any of a mark's markers reads, one glyph a level, as
// typing does.
func inlineLevel(kind model.MarkKind, text string) (uint8, bool) {
	if text == "" {
		return 0, true
	}
	desc := describeMark(kind)
	if !strings.ContainsRune(desc.Glyphs, rune(text[0])) || strings.Trim(text, text[:1]) != "" || len(text) > int(desc.Top) {
		return 0, false
	}
	return uint8(len(text)), true
}

// markStyle adds what a mark shows to style.
func markStyle(style *surface.TextStyle, mark model.InlineMark) {
	if desc := markOf(mark); desc.style != nil {
		desc.style(mark.Level(), style)
	}
}

// markRole is the role of a mark's notation opening it, or closing it.
func markRole(mark model.InlineMark, opening bool) projection.Role {
	desc := markOf(mark)
	switch {
	case opening && desc.open != 0:
		return desc.open
	case opening:
		return roleMarkOpen
	case desc.close != 0:
		return desc.close
	}
	return roleMarkClose
}

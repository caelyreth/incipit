package editor

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// The kinds of node a page holds. Each is described once, below, by what it
// is: everything else in the editor asks a kind's descriptor, never the
// kind's name, so a new kind is one more entry here (kinds_test.go keeps
// it so).
const (
	// DocumentNode is the kind of a page's root.
	DocumentNode model.NodeTypeID = "document"
	// ParagraphNode is the plain block every page starts with.
	ParagraphNode model.NodeTypeID = "paragraph"
	// HeadingNode is a heading block; its "level" attribute is 1 to 6.
	HeadingNode model.NodeTypeID = "heading"
	// QuoteNode is a block quote.
	QuoteNode model.NodeTypeID = "quote"
	// BulletNode is an item of a bullet list.
	BulletNode model.NodeTypeID = "bullet"
	// CheckboxNode is a list item whose checked state belongs to the page.
	CheckboxNode model.NodeTypeID = "checkbox"
	// OrderedNode is an item of an ordered list, numbered by its place in
	// the run of items it stands in.
	OrderedNode model.NodeTypeID = "ordered"
	// CodeNode is a fenced code block; its text keeps its line breaks and
	// its "info" attribute names the language.
	CodeNode model.NodeTypeID = "code"
	// MathNode is a display formula; its text is the formula's TeX source,
	// kept line for line as code's is.
	MathNode model.NodeTypeID = "math"
	// RuleNode is a thematic break.
	RuleNode model.NodeTypeID = "rule"
	// ImageNode is an image block; its text is its caption.
	ImageNode model.NodeTypeID = "image"
	// CellNode is one cell of a table; its text is what the cell holds.
	CellNode model.NodeTypeID = "cell"
)

// KindDesc is what a kind of node is, as the editor treats it.
type KindDesc struct {
	Kind model.NodeTypeID
	// Root is the page itself, which holds the blocks.
	Root bool
	// Plain is the block typing reshapes and every edit falls back to.
	Plain bool
	// Leveled blocks store a level, as a heading's 1 to 6.
	Leveled bool
	// Group is the family of runs a block stands in with its neighbours,
	// nested by depth; nil for a block standing alone.
	Group *GroupDesc
	// Raise is what the block's own glyph typed at its marker raises.
	Raise Raise
	// Box is the state the block's marker holds in a box, if it holds one.
	Box *BoxGrammar
	// Marker is the role of the glyphs of the marker opening the block,
	// its token, which the caret stops in (tokens.go).
	Marker projection.Role
	// Prefix notation opens the block's line, its text following on it,
	// rather than a line of its own.
	Prefix bool
	// Numbered items count their place in the run they stand in.
	Numbered bool
	// Checks holds a checked state the page keeps.
	Checks bool
	// Checkable blocks become checkboxes when a box is typed at their start.
	Checkable bool
	// Literal text is taken as written: code, a formula's source.
	Literal bool
	// Info blocks name what their literal text is, as code its language.
	Info bool
	// Formula text is TeX.
	Formula bool
	// KeepsLines text holds line breaks of its own rather than breaking
	// into blocks at them.
	KeepsLines bool
	// Atomic blocks stand on an object, their text its caption.
	Atomic bool
	// Divider blocks are a line with no text of their own.
	Divider bool
	// Cell blocks stand in a table's grid.
	Cell bool
	// BreakLeaves blocks end at a break: what follows is a paragraph.
	BreakLeaves bool
	// Label names the kind where a block's modifier control offers it,
	// and Offers how many of its levels it offers there.
	Label  string
	Offers int
	// Glyphs typed at a block's token make it this kind, or step its level
	// when it is this kind already.
	Glyphs string

	rhythm   surface.Rhythm
	align    surface.BlockAlign
	style    func(blockShape, *surface.TextStyle)
	notation func(shape blockShape, ordinal int) (open, close notation)
	opens    func(writingTriggers) bool
	read     func(*model.Node, *blockShape)
	// parse reads notation opening a block, unnested, as this kind's shape.
	parse func(text string) (blockShape, bool)
}

// GroupDesc is a family of blocks standing in runs with their neighbours,
// nested in one another by depth.
type GroupDesc struct {
	Family string
	// Indented groups nest by an indent before the marker, as lists do;
	// others by repeating the marker, as quotes do.
	Indented bool
	// Counts numbered items in the group by their place in its run.
	Counts bool
}

var (
	listGroup  = &GroupDesc{Family: "list", Indented: true, Counts: true}
	quoteGroup = &GroupDesc{Family: "quote"}
)

// Raise is what a block's own glyph typed at its marker raises.
type Raise uint8

const (
	// RaiseNothing leaves the block as it is: another of a list's markers.
	RaiseNothing Raise = iota
	// RaiseLevel steps the block's level up, as "#" a heading's.
	RaiseLevel
	// RaiseDepth nests the block a level deeper, as ">" a quote line.
	RaiseDepth
)

// BoxGrammar is a state a block's marker holds in a box between Open and
// Close: Clear's first glyph shows it unset, Set's set, and any of either
// typed in the box reads as that state.
type BoxGrammar struct {
	Open, Close rune
	Clear, Set  string
	// Label names the set state where a block's modifier control offers
	// it.
	Label string
}

// reads is the state glyph r typed in the box sets.
func (b *BoxGrammar) reads(r rune) (set, ok bool) {
	switch {
	case strings.ContainsRune(b.Set, r):
		return true, true
	case strings.ContainsRune(b.Clear, r):
		return false, true
	}
	return false, false
}

// String is the box showing a state.
func (b *BoxGrammar) show(set bool) string {
	state := b.Clear[:1]
	if set {
		state = b.Set[:1]
	}
	return string(b.Open) + state + string(b.Close)
}

// parse reads a box at the start of text, reporting its state and length.
func (b *BoxGrammar) parse(text string) (set bool, length int, ok bool) {
	runes := []rune(text)
	if len(runes) < 3 || runes[0] != b.Open || runes[2] != b.Close {
		return false, 0, false
	}
	set, ok = b.reads(runes[1])
	return set, len(string(runes[:3])), ok
}

var checkBox = &BoxGrammar{Open: '[', Close: ']', Clear: " ", Set: "xX", Label: "Checked"}

// listMarker reads a list item's marker, "-" or "+", and the space after.
func listMarker(text string) (string, bool) {
	if len(text) >= 2 && (text[0] == '-' || text[0] == '+') && text[1] == ' ' {
		return text[2:], true
	}
	return "", false
}

var kindTable = []KindDesc{
	{Kind: DocumentNode, Root: true},
	{Kind: ParagraphNode, Plain: true, Label: "Text"},
	{
		Kind: HeadingNode, Label: "Heading", Offers: 3, Leveled: true, Prefix: true, BreakLeaves: true, Glyphs: "#", Raise: RaiseLevel, Marker: roleHeading,
		parse: func(text string) (blockShape, bool) {
			hashes := strings.TrimSuffix(text, " ")
			if hashes == text || hashes == "" || len(hashes) > 6 || strings.Trim(hashes, "#") != "" {
				return blockShape{}, false
			}
			return blockShape{kind: HeadingNode, level: len(hashes)}, true
		},
		rhythm: surface.RhythmTitle,
		style: func(shape blockShape, style *surface.TextStyle) {
			style.Size, style.Bold, style.Color = titleSize(shape.level), true, surface.ColorStrong
		},
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{{Text: strings.Repeat("#", shape.level), Role: roleHeading}, spacePart}, nil
		},
		opens: func(t writingTriggers) bool { return t.headings },
		read: func(node *model.Node, shape *blockShape) {
			shape.level = 1
			if value, ok := node.Attribute(headingLevelAttribute); ok {
				if level, ok := value.Integer(); ok {
					shape.level = int(max(1, min(6, level)))
				}
			}
		},
	},
	{
		Kind: QuoteNode, Label: "Quote", Group: quoteGroup, Prefix: true, Glyphs: ">", Raise: RaiseDepth, Marker: roleQuote,
		parse: func(text string) (blockShape, bool) {
			quotes := strings.Count(text, "> ")
			if quotes == 0 || quotes-1 > maxDepth || text != strings.Repeat("> ", quotes) {
				return blockShape{}, false
			}
			return blockShape{kind: QuoteNode, depth: quotes - 1}, true
		},
		rhythm: surface.RhythmEntry,
		style:  func(_ blockShape, style *surface.TextStyle) { style.Italic = true },
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{{Text: strings.Repeat("> ", shape.depth+1), Role: roleQuote}}, nil
		},
		opens: func(t writingTriggers) bool { return t.quotes },
	},
	{
		Kind: BulletNode, Label: "Bulleted List", Group: listGroup, Prefix: true, Checkable: true, Glyphs: "-*+", Marker: roleBullet,
		parse: func(text string) (blockShape, bool) {
			rest, ok := listMarker(text)
			return blockShape{kind: BulletNode}, ok && rest == ""
		},
		rhythm: surface.RhythmEntry,
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{indentPart(shape), {Text: "-", Role: roleBullet}, spacePart}, nil
		},
		opens: func(t writingTriggers) bool { return t.lists },
	},
	{
		Kind: CheckboxNode, Label: "Checklist", Group: listGroup, Prefix: true, Checks: true, Box: checkBox, Marker: roleCheckbox,
		rhythm: surface.RhythmEntry,
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{indentPart(shape), {Text: "- " + checkBox.show(shape.checked), Role: roleCheckbox}, spacePart}, nil
		},
		parse: func(text string) (blockShape, bool) {
			rest, ok := listMarker(text)
			if !ok {
				return blockShape{}, false
			}
			checked, length, ok := checkBox.parse(rest)
			return blockShape{kind: CheckboxNode, checked: checked}, ok && rest[length:] == " "
		},
		opens: func(t writingTriggers) bool { return t.lists },
		read: func(node *model.Node, shape *blockShape) {
			if value, ok := node.Attribute(checkedAttribute); ok {
				shape.checked, _ = value.Bool()
			}
		},
	},
	{
		Kind: OrderedNode, Label: "Numbered List", Group: listGroup, Prefix: true, Numbered: true, Glyphs: "0123456789", Marker: roleOrdinal,
		parse: func(text string) (blockShape, bool) {
			digits := strings.TrimSuffix(text, ". ")
			ok := digits != text && digits != "" && len(digits) <= 9 && strings.Trim(digits, "0123456789") == ""
			return blockShape{kind: OrderedNode}, ok
		},
		rhythm: surface.RhythmEntry,
		notation: func(shape blockShape, ordinal int) (notation, notation) {
			return notation{indentPart(shape), {Text: strconv.Itoa(ordinal) + ".", Role: roleOrdinal}, spacePart}, nil
		},
		opens: func(t writingTriggers) bool { return t.lists },
	},
	{
		Kind: CodeNode, Literal: true, Info: true, KeepsLines: true,
		rhythm: surface.RhythmVerbatim,
		style:  func(_ blockShape, style *surface.TextStyle) { codeStyle(style) },
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{{Text: "```" + shape.info + "\n", Role: roleCodeOpen}}, notation{{Text: "\n```", Role: roleCodeClose}}
		},
		parse: func(text string) (blockShape, bool) {
			if !strings.HasPrefix(text, "```") || !strings.HasSuffix(text, "\n") || strings.ContainsAny(text[3:len(text)-1], "`\n") {
				return blockShape{}, false
			}
			return blockShape{kind: CodeNode, info: strings.TrimSpace(text[3 : len(text)-1])}, true
		},
		opens: func(t writingTriggers) bool { return t.code },
		read: func(node *model.Node, shape *blockShape) {
			if value, ok := node.Attribute(codeInfoAttribute); ok {
				shape.info, _ = value.Text()
			}
		},
	},
	{
		Kind: MathNode, Literal: true, Formula: true, KeepsLines: true,
		rhythm: surface.RhythmVerbatim, align: surface.AlignCenter,
		style: func(_ blockShape, style *surface.TextStyle) { codeStyle(style) },
		notation: func(blockShape, int) (notation, notation) {
			return notation{{Text: "$$\n", Role: roleFormulaOpen}}, notation{{Text: "\n$$", Role: roleFormulaClose}}
		},
		parse: func(text string) (blockShape, bool) { return blockShape{kind: MathNode}, text == "$$\n" },
		opens: func(t writingTriggers) bool { return t.math },
	},
	{
		Kind: RuleNode, Divider: true,
		style: func(_ blockShape, style *surface.TextStyle) { style.Color = surface.ColorSubtle },
		notation: func(blockShape, int) (notation, notation) {
			return notation{{Text: "---", Role: roleRule}}, nil
		},
		parse: func(text string) (blockShape, bool) { return blockShape{kind: RuleNode}, text == "---" },
		opens: func(t writingTriggers) bool { return t.rules },
	},
	{
		Kind: ImageNode, Atomic: true, BreakLeaves: true,
		style: func(_ blockShape, style *surface.TextStyle) {
			style.Size, style.Color = surface.SizeCaption, surface.ColorMuted
		},
		notation: func(blockShape, int) (notation, notation) {
			return notation{{Text: figureNotation, Role: roleFigure}}, nil
		},
	},
	{Kind: CellNode, Cell: true, KeepsLines: true},
}

var kindIndex = func() map[model.NodeTypeID]*KindDesc {
	index := make(map[model.NodeTypeID]*KindDesc, len(kindTable))
	for i := range kindTable {
		index[kindTable[i].Kind] = &kindTable[i]
	}
	return index
}()

// unknownKind describes a kind the editor does not know: nothing.
var unknownKind KindDesc

// describe returns what kind is.
func describe(kind model.NodeTypeID) *KindDesc {
	if desc, ok := kindIndex[kind]; ok {
		return desc
	}
	return &unknownKind
}

// kindOf returns what node is; a missing node is nothing.
func kindOf(node *model.Node) *KindDesc {
	if node == nil {
		return &unknownKind
	}
	return describe(node.Kind())
}

// Describe returns what kind of node the editor takes kind for.
func Describe(kind model.NodeTypeID) KindDesc { return *describe(kind) }

// plainKind is the kind of block every edit falls back to.
func plainKind() model.NodeTypeID {
	for _, desc := range kindTable {
		if desc.Plain {
			return desc.Kind
		}
	}
	return ""
}

// boxedKind is the kind of group whose marker holds a box.
func boxedKind(group *GroupDesc) (*KindDesc, bool) {
	for i := range kindTable {
		if group != nil && kindTable[i].Group == group && kindTable[i].Box != nil {
			return &kindTable[i], true
		}
	}
	return nil, false
}

// markerKind is the kind whose marker plays role, if one's does.
func markerKind(role projection.Role) (*KindDesc, bool) {
	for i := range kindTable {
		if role != 0 && kindTable[i].Marker == role {
			return &kindTable[i], true
		}
	}
	return nil, false
}

// desc is what a shape's kind is.
func (s blockShape) desc() *KindDesc { return describe(s.kind) }

var spacePart = projection.Part{Text: " ", Role: roleSpace}

func indentPart(shape blockShape) projection.Part {
	return projection.Part{Text: strings.Repeat("  ", shape.depth), Role: roleIndent}
}

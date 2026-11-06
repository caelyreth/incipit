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
	// Group is the run of neighbours a block stands in, nested by depth:
	// "list" or "quote"; empty for a block standing alone.
	Group string
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

	rhythm   surface.Rhythm
	align    surface.BlockAlign
	style    func(blockShape, *surface.TextStyle)
	notation func(shape blockShape, ordinal int) (open, close notation)
	opens    func(writingTriggers) bool
	read     func(*model.Node, *blockShape)
}

var kindTable = []KindDesc{
	{Kind: DocumentNode, Root: true},
	{Kind: ParagraphNode, Plain: true},
	{
		Kind: HeadingNode, Leveled: true, Prefix: true, BreakLeaves: true,
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
		Kind: QuoteNode, Group: "quote", Prefix: true,
		rhythm: surface.RhythmEntry,
		style:  func(_ blockShape, style *surface.TextStyle) { style.Italic = true },
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{{Text: strings.Repeat("> ", shape.depth+1), Role: roleQuote}}, nil
		},
		opens: func(t writingTriggers) bool { return t.quotes },
	},
	{
		Kind: BulletNode, Group: "list", Prefix: true, Checkable: true,
		rhythm: surface.RhythmEntry,
		notation: func(shape blockShape, _ int) (notation, notation) {
			return notation{indentPart(shape), {Text: "-", Role: roleBullet}, spacePart}, nil
		},
		opens: func(t writingTriggers) bool { return t.lists },
	},
	{
		Kind: CheckboxNode, Group: "list", Prefix: true, Checks: true,
		rhythm: surface.RhythmEntry,
		notation: func(shape blockShape, _ int) (notation, notation) {
			mark := " "
			if shape.checked {
				mark = "x"
			}
			return notation{indentPart(shape), {Text: "- [" + mark + "]", Role: roleCheckbox}, spacePart}, nil
		},
		opens: func(t writingTriggers) bool { return t.lists },
		read: func(node *model.Node, shape *blockShape) {
			if value, ok := node.Attribute(checkedAttribute); ok {
				shape.checked, _ = value.Bool()
			}
		},
	},
	{
		Kind: OrderedNode, Group: "list", Prefix: true, Numbered: true,
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
		opens: func(t writingTriggers) bool { return t.math },
	},
	{
		Kind: RuleNode, Divider: true,
		style: func(_ blockShape, style *surface.TextStyle) { style.Color = surface.ColorSubtle },
		notation: func(blockShape, int) (notation, notation) {
			return notation{{Text: "---", Role: roleRule}}, nil
		},
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

// desc is what a shape's kind is.
func (s blockShape) desc() *KindDesc { return describe(s.kind) }

var spacePart = projection.Part{Text: " ", Role: roleSpace}

func indentPart(shape blockShape) projection.Part {
	return projection.Part{Text: strings.Repeat("  ", shape.depth), Role: roleIndent}
}

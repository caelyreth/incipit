package editor

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel/projection"
)

// The roles the Writing Profile's notation plays, which the projection
// carries for it: what reads the page asks which run of notation is a
// checkbox or a formula's fence, rather than measuring the notation's
// spelling.
const (
	roleIndent projection.Role = iota + 1
	roleHeading
	roleQuote
	roleBullet
	roleOrdinal
	roleCheckbox
	// roleSpace is the space a block's marker ends with.
	roleSpace
	roleCodeOpen
	roleCodeClose
	roleFormulaOpen
	roleFormulaClose
	roleRule
	roleFigure
	roleMarkOpen
	roleMarkClose
)

// figureNotation is the one character an atomic block, a picture, shows
// as: an object, which the page sets the picture above.
const figureNotation = "￼"

// notation is generated notation as the parts it is made of, each with the
// role it plays.
type notation []projection.Part

func (n notation) String() string {
	var text strings.Builder
	for _, part := range n {
		text.WriteString(part.Text)
	}
	return text.String()
}

// blockNotationParts is the notation a block of shape opens and closes
// with, ordinal numbering an ordered item.
func blockNotationParts(shape blockShape, ordinal int) (open, close notation) {
	if shape.atomic() {
		return notation{{Text: figureNotation, Role: roleFigure}}, nil
	}
	indent := projection.Part{Text: strings.Repeat("  ", shape.depth), Role: roleIndent}
	space := projection.Part{Text: " ", Role: roleSpace}
	switch shape.kind {
	case HeadingNode:
		return notation{{Text: strings.Repeat("#", shape.level), Role: roleHeading}, space}, nil
	case QuoteNode:
		return notation{{Text: strings.Repeat("> ", shape.depth+1), Role: roleQuote}}, nil
	case BulletNode:
		return notation{indent, {Text: "-", Role: roleBullet}, space}, nil
	case CheckboxNode:
		mark := " "
		if shape.checked {
			mark = "x"
		}
		return notation{indent, {Text: "- [" + mark + "]", Role: roleCheckbox}, space}, nil
	case OrderedNode:
		return notation{indent, {Text: strconv.Itoa(ordinal) + ".", Role: roleOrdinal}, space}, nil
	case CodeNode:
		return notation{{Text: "```" + shape.info + "\n", Role: roleCodeOpen}}, notation{{Text: "\n```", Role: roleCodeClose}}
	case MathNode:
		return notation{{Text: "$$\n", Role: roleFormulaOpen}}, notation{{Text: "\n$$", Role: roleFormulaClose}}
	case RuleNode:
		return notation{{Text: "---", Role: roleRule}}, nil
	}
	return nil, nil
}

// blockNotation is the notation a block of shape opens and closes with,
// as text.
func blockNotation(shape blockShape, ordinal int) (open, close string) {
	opening, closing := blockNotationParts(shape, ordinal)
	return opening.String(), closing.String()
}

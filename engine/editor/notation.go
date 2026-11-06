package editor

import (
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
	if desc := shape.desc(); desc.notation != nil {
		return desc.notation(shape, ordinal)
	}
	return nil, nil
}

// blockNotation is the notation a block of shape opens and closes with,
// as text.
func blockNotation(shape blockShape, ordinal int) (open, close string) {
	opening, closing := blockNotationParts(shape, ordinal)
	return opening.String(), closing.String()
}

package vocab

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
)

// Spelling is one way to write a mark or a block marker.
type Spelling string

// Notation sets: the spellings of marks and block markers, which ghost
// notation reveals and the Markdown policy reads. The first spelling of
// each is the one notation shows by default.
var (
	EmphasisSpellings = []Spelling{"*", "_"}
	StrongSpellings   = []Spelling{"**", "__"}
	CodeSpellings     = []Spelling{"`"}
	StrikeSpellings   = []Spelling{"~"}

	QuoteSpelling    Spelling = ">"
	BulletSpellings           = []Spelling{"-", "*", "+"}
	TaskOpenSpelling Spelling = "[ ]"
	TaskDoneSpelling Spelling = "[x]"
	FenceSpelling    Spelling = "```"
	FormulaSpelling  Spelling = "$"
	MathSpelling     Spelling = "$$"
	DividerSpelling  Spelling = "---"
)

// MarkSpellings returns the spellings of a mark kind.
func MarkSpellings(k doc.MarkKind) []Spelling {
	switch k {
	case Emphasis:
		return EmphasisSpellings
	case Strong:
		return StrongSpellings
	case CodeMark:
		return CodeSpellings
	case Strike:
		return StrikeSpellings
	}
	return nil
}

// MarkOfSpelling returns the mark a delimiter spells.
func MarkOfSpelling(s Spelling) (doc.MarkKind, bool) {
	for _, k := range []doc.MarkKind{Strong, Emphasis, CodeMark, Strike} {
		for _, sp := range MarkSpellings(k) {
			if sp == s {
				return k, true
			}
		}
	}
	return 0, false
}

// HeadingSpelling is the marker of a heading level: "#" repeated.
func HeadingSpelling(level int) Spelling { return Spelling(strings.Repeat("#", level)) }

// OrderedSpelling is the marker of an ordered item numbered n.
func OrderedSpelling(n int) Spelling { return Spelling(strconv.Itoa(n) + ".") }

// ParseOrdered reads an ordered marker such as "12." and its number.
func ParseOrdered(s string) (int, bool) {
	if len(s) < 2 || s[len(s)-1] != '.' {
		return 0, false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	return n, err == nil && n >= 0
}

package vocab

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
)

// Spelling is one way to write a mark or a block marker.
type Spelling string

// Notation sets: the spellings of marks and block markers, which ghost
// notation shows and the Markdown policy reads. The first spelling of
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

// Marker is what a block's structure spells before its text: a heading's
// level, or an item's list style, number and task. Notation always spells
// it as a ghost; it is the one marker a block has (§13.6).
type Marker struct {
	Kind    doc.Kind
	Level   int
	Style   ListStyle
	Ordinal int
	Task    Task
}

// MarkerOf is the marker of block n, where its structure spells one.
func MarkerOf(r doc.Reader, n *doc.Node) (Marker, bool) {
	switch n.Kind() {
	case Heading:
		return Marker{Kind: Heading, Level: HeadingLevel(n)}, true
	case Item:
		m := Marker{Kind: Item, Task: TaskOf(n)}
		list, slot, index, ok := Index(r, n.ID())
		if !ok {
			return Marker{}, false
		}
		ln, _ := r.Node(list)
		if ln.Slot(slot).IsInline() {
			return Marker{}, false
		}
		m.Style, m.Ordinal = ListStyleOf(ln), ListStart(ln)+index
		return m, true
	}
	return Marker{}, false
}

// Spelling is the marker as notation spells it, with the separator after
// it: "## ", "- ", "3. ", "- [ ] ".
func (m Marker) Spelling() Spelling {
	if m.Kind == Heading {
		return HeadingSpelling(m.Level) + " "
	}
	s := BulletSpellings[0]
	if m.Style == Ordered {
		s = OrderedSpelling(m.Ordinal)
	}
	switch m.Task {
	case TaskOpen:
		s += " " + TaskOpenSpelling
	case TaskDone:
		s += " " + TaskDoneSpelling
	}
	return s + " "
}

// Glyphs is how many caret stops the marker holds: one after each of its
// glyphs, its final separator being a gap.
func (m Marker) Glyphs() int {
	return GraphemeCount(strings.TrimSuffix(string(m.Spelling()), " "))
}

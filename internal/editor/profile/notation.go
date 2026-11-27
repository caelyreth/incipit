package profile

import (
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Spelling is vocab.Spelling, one way to write a mark or a marker.
type Spelling = vocab.Spelling

// Notation sets: the spellings of marks and block markers, which ghost
// notation shows and the Markdown policy reads. The first spelling of
// each is the one notation shows by default.
var (
	EmphasisSpellings = []Spelling{"*", "_"}
	StrongSpellings   = []Spelling{"**", "__"}
	CodeSpellings     = []Spelling{"`"}
	StrikeSpellings   = []Spelling{"~"}

	QuoteSpelling          Spelling = ">"
	BulletSpellings                 = []Spelling{"-", "*", "+"}
	OrderedDelimiters               = []byte{'.', ')'}
	HeadingMarker          Spelling = "#"
	TaskOpenSpelling       Spelling = "[ ]"
	TaskDoneSpelling       Spelling = "[x]"
	TaskDoneAltSpelling    Spelling = "[X]"
	FenceSpelling          Spelling = "```"
	FormulaSpelling        Spelling = "$"
	MathSpelling           Spelling = "$$"
	DividerSpelling        Spelling = "---"
	RuleChars                       = "-*_"
	StrikeMarkdownSpelling Spelling = "~~"
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

// SpelledMarks are the marks notation spells, innermost first: where two
// marks' signs stand at one offset with equal runs, the inner one's sign
// stands nearer its text.
var SpelledMarks = []doc.MarkKind{CodeMark, Strike, Emphasis, Strong}

// MarkLevels are marks that are levels of one another, lowest first: one
// more delimiter spells the next level.
var MarkLevels = [][]doc.MarkKind{{Emphasis, Strong}}

// MarkStep is the mark one level from k in dir: below the lowest level, or
// for a mark with no levels, none (zero); false where nothing is above.
func MarkStep(k doc.MarkKind, dir vocab.Direction) (doc.MarkKind, bool) {
	for _, levels := range MarkLevels {
		if i := slices.Index(levels, k); i >= 0 {
			switch j := i + int(dir); {
			case j < 0:
				return 0, true
			case j >= len(levels):
				return 0, false
			default:
				return levels[j], true
			}
		}
	}
	return 0, dir < 0
}

// MarkOfSpelling returns the mark a delimiter spells.
func MarkOfSpelling(s Spelling) (doc.MarkKind, bool) {
	for _, k := range SpelledMarks {
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
// level, an item's list style, number and task, or a code block's fence
// and its language. Notation always spells it as a ghost; it is the one
// marker a block has (§13.6).
type Marker struct {
	Kind    doc.Kind
	Level   int
	Style   ListStyle
	Ordinal int
	Task    Task
	// Info is a code block's info string, its language first.
	Info string
}

// MarkerOf is the marker of block n, where its structure spells one.
func MarkerOf(r doc.Reader, n *doc.Node) (Marker, bool) {
	switch n.Kind() {
	case Heading:
		return Marker{Kind: Heading, Level: HeadingLevel(n)}, true
	case Code:
		return Marker{Kind: Code, Info: CodeInfo(n)}, true
	case Item:
		m := Marker{Kind: Item, Task: TaskOf(n)}
		list, slot, index, ok := vocab.Index(r, n.ID())
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
// it: "## ", "- ", "3. ", "- [ ] ", and a code block's fence on a line of
// its own, "```go\n".
func (m Marker) Spelling() Spelling {
	switch m.Kind {
	case Heading:
		return HeadingSpelling(m.Level) + " "
	case Code:
		return FenceSpelling + Spelling(m.Info) + "\n"
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

// Glyphs is how many glyphs the marker spells before its separator.
func (m Marker) Glyphs() int {
	return vocab.GraphemeCount(string(m.Spelling())) - 1
}

// ClosingFence is how notation spells the end of a code block: its fence
// on a line of its own.
var ClosingFence = "\n" + FenceSpelling

// CodeInfo is a code block's info string.
func CodeInfo(n *doc.Node) string {
	return Schema().Attr(n, FieldInfo).Text()
}

// CodeLanguage is the language a code block's info string names: its
// first word, lower-cased.
func CodeLanguage(info string) string {
	lang, _, _ := strings.Cut(strings.TrimSpace(info), " ")
	return strings.ToLower(lang)
}

// ValidInfo reports text a code block's info string may take: one line,
// and no backtick, which would end its fence.
func ValidInfo(text string) bool {
	for _, r := range text {
		if r == '`' || r < ' ' || r == 0x7f {
			return false
		}
	}
	return true
}

// TableRow reads a line written as a table's row, "| a | b |", as its
// cells' texts; nil when it is none. A pipe escaped as "\|" is text.
func TableRow(line string) []string {
	t := strings.TrimSpace(line)
	if len(t) < 2 || t[0] != '|' || t[len(t)-1] != '|' || strings.HasSuffix(t, `\|`) {
		return nil
	}
	var cells []string
	var b strings.Builder
	for i := 1; i < len(t); i++ {
		switch {
		case t[i] == '\\' && i+1 < len(t) && t[i+1] == '|':
			b.WriteByte('|')
			i++
		case t[i] == '|':
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteByte(t[i])
		}
	}
	return cells
}

// TableAligns reads a table's delimiter row, "| --- | :-: | --: |", as its
// columns' alignments; false when the line is none.
func TableAligns(line string) ([]Align, bool) {
	cells := TableRow(line)
	if cells == nil {
		return nil, false
	}
	aligns := make([]Align, len(cells))
	for i, c := range cells {
		dashes := strings.Trim(c, ":")
		if dashes == "" || strings.Trim(dashes, "-") != "" {
			return nil, false
		}
		switch left, right := strings.HasPrefix(c, ":"), strings.HasSuffix(c, ":"); {
		case left && right:
			aligns[i] = AlignCenter
		case right:
			aligns[i] = AlignEnd
		}
	}
	return aligns, true
}

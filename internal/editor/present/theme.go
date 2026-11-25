package present

import "github.com/caelyreth/neverland/internal/editor/vocab"

// FontRole is a theme font role.
type FontRole uint8

const (
	FontBody FontRole = iota
	FontHeading
	FontMono
	FontMath
)

// Field bits say which attributes a style sets.
const (
	SetFont uint32 = 1 << iota
	SetSize
	SetWeight
	SetItalic
	SetColor
	SetBackground
	SetLine
	SetStrike
	SetOpacity
	SetLineColor
	SetSpace
	SetOutline
	SetAfter
)

// LineKind is the line a style draws under its text.
type LineKind uint8

const (
	LineNone LineKind = iota
	// LineUnder is a straight underline, as composing text shows.
	LineUnder
	// LineBox is a box around the text, drawn as an object it holds is
	// outlined: what the next delete takes, wherever it is.
	LineBox
	// LineWave is a wavy underline, as a spell checker marks a word.
	LineWave
)

// Style is a closed set of style attributes. Layout attributes (font role,
// size, weight, italic, the room around a unit) and paint attributes
// (colour, background, line and its color, strike, opacity) are cached
// apart by the renderer: a change to paint attributes never invalidates
// layout.
type Style struct {
	Set        uint32
	Font       FontRole
	Size       float32 // a scale of the body size
	Weight     int
	Italic     bool
	Color      uint32 // 0xRRGGBBAA
	Background uint32
	Line       LineKind
	// LineColor is the line's color, 0xRRGGBBAA; unset, the text's.
	LineColor uint32
	Strike    bool
	Opacity   float32
	// Space is the room before a unit of the group, in gaps; unset, the
	// page's own. After is the room it keeps after it: two units stand
	// as far apart as the larger of the upper's After and the lower's
	// Space asks.
	Space, After float32
	// Outline is how a decoration marks an object it holds whole, which
	// it draws around rather than over, 0xRRGGBBAA; unset, its background.
	Outline uint32
}

// Theme maps highlight groups to styles.
type Theme map[vocab.Group]Style

func (s *Style) over(o Style) {
	if o.Set&SetFont != 0 {
		s.Font = o.Font
	}
	if o.Set&SetSize != 0 {
		s.Size = o.Size
	}
	if o.Set&SetWeight != 0 {
		s.Weight = o.Weight
	}
	if o.Set&SetItalic != 0 {
		s.Italic = o.Italic
	}
	if o.Set&SetColor != 0 {
		s.Color = o.Color
	}
	if o.Set&SetBackground != 0 {
		s.Background = o.Background
	}
	if o.Set&SetLine != 0 {
		s.Line = o.Line
	}
	if o.Set&SetLineColor != 0 {
		s.LineColor = o.LineColor
	}
	if o.Set&SetStrike != 0 {
		s.Strike = o.Strike
	}
	if o.Set&SetOpacity != 0 {
		s.Opacity = o.Opacity
	}
	if o.Set&SetSpace != 0 {
		s.Space = o.Space
	}
	if o.Set&SetOutline != 0 {
		s.Outline = o.Outline
	}
	if o.Set&SetAfter != 0 {
		s.After = o.After
	}
	s.Set |= o.Set
}

// Resolve styles groups, outermost first; each group applies its fallback
// chain from the root down, so a policy group inherits its vocabulary
// group.
func (t Theme) Resolve(groups []vocab.Group) Style {
	s := Style{Size: 1, Weight: 400, Opacity: 1}
	for _, g := range groups {
		var chain []vocab.Group
		for c := g; c != 0; c = c.Fallback() {
			chain = append(chain, c)
		}
		for i := len(chain) - 1; i >= 0; i-- {
			if st, ok := t[chain[i]]; ok {
				s.over(st)
			}
		}
	}
	return s
}

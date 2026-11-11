package manifest

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policies/notation"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// shown renders the unit holding the caret as the view presents it:
// ghosts in brackets, and the caret as "|" at its stop. At an offset, the
// place before the first ghost is its StopFrom, each glyph boundary a
// stop of its own, and the place after each ghost its StopTo; an offset
// without ghosts has only its own stop, 0.
func (p *page) shown() string {
	p.t.Helper()
	v, _ := p.e.View(1)
	head := v.Selection.Head
	pres := p.e.Present(1, []doc.ID{head.Pos.Node})[0]
	if pres == nil || len(pres.Surfaces) == 0 {
		p.t.Fatalf("no presentation for %v", head)
	}
	items := pres.Surfaces[0].Flow.Items
	first := map[int]int{}
	for _, it := range items {
		if _, ok := first[it.From]; !ok && it.Kind == present.GhostItem {
			first[it.From] = it.StopFrom
		}
	}
	var b strings.Builder
	at := func(off, stop int) bool { return head.Pos.Offset == off && head.Stop == stop }
	// The place before everything at an offset is written once.
	wrote := map[int]bool{}
	before := func(off int) {
		if !wrote[off] && at(off, first[off]) {
			b.WriteByte('|')
		}
		wrote[off] = true
	}
	end := 0
	for _, it := range items {
		switch it.Kind {
		case present.GhostItem:
			before(it.From)
			b.WriteByte('[')
			stops := it.StopTo - it.StopFrom
			for i, r := range []rune(string(it.Spelling)) {
				if i > 0 && i < stops && at(it.From, it.StopFrom+i) {
					b.WriteByte('|')
				}
				b.WriteRune(r)
			}
			b.WriteByte(']')
			if at(it.From, it.StopTo) {
				b.WriteByte('|')
			}
		case present.TextRun:
			for i := range len(it.Text) {
				before(it.From + i)
				b.WriteByte(it.Text[i])
			}
		case present.BreakItem:
			before(it.From)
			b.WriteString("⏎")
		}
		end = max(end, it.To)
	}
	before(end)
	return b.String()
}

func (p *page) wantShown(s string) {
	p.t.Helper()
	if got := p.shown(); got != s {
		p.t.Fatalf("shown %q, want %q\n%s", got, s, outline(p.e.Snapshot().Root()))
	}
}

func (p *page) head() doc.Caret {
	v, _ := p.e.View(1)
	return v.Selection.Head
}

func TestClosingARunLeavesTheCaretAfterItsGhost(t *testing.T) {
	p := open(t)
	p.typeText("a **bold**")
	p.wantShown("a [**]bold[**]|")
	p.typeText("x")
	p.want("page(paragraph(a <bold>x))")
	p.wantShown("a [**]bold[**]x|")
}

// Notation looks the same wherever the caret is; hiding it hides mark
// notation only, never a block's marker.
func TestNotationDoesNotDependOnTheCaret(t *testing.T) {
	ghosts := func(reveal string) (spelled string) {
		p := revealed(t, reveal)
		p.typeText("# a *em*")
		p.do(vocab.Break{})
		p.typeText("next")
		first := p.e.Snapshot().Root().Slot(0).Child(0).ID()
		for _, it := range p.e.Present(1, []doc.ID{first})[0].Surfaces[0].Flow.Items {
			if it.Kind == present.GhostItem {
				spelled += string(it.Spelling)
			}
		}
		return spelled
	}
	if got := ghosts(notation.RevealShown); got != "# **" {
		t.Fatalf("the heading the caret left spells %q, want \"# **\"", got)
	}
	if got := ghosts(notation.RevealHidden); got != "# " {
		t.Fatalf("hidden notation spells %q, want the marker alone", got)
	}
}

func TestStopsStepThroughGhostGlyphs(t *testing.T) {
	p := open(t)
	p.typeText("a **b**")
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	steps := []string{"a [**]b[*|*]", "a [**]b|[**]", "a [**]|b[**]", "a [*|*]b[**]", "a |[**]b[**]", "a| [**]b[**]"}
	for _, want := range steps {
		p.do(left)
		p.wantShown(want)
	}
	right := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
	for i := len(steps) - 2; i >= 0; i-- {
		p.do(right)
		p.wantShown(steps[i])
	}
	p.do(right)
	p.wantShown("a [**]b[**]|")
}

func TestAStopDecidesTheMarksTypedThere(t *testing.T) {
	p := open(t)
	p.typeText("**b**")
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	// Before the closing ghost the text is still bold.
	p.do(left)
	p.do(left)
	p.wantShown("[**]b|[**]")
	p.typeText("c")
	p.want("page(paragraph(<bc>))")
	// Before the opening ghost it is not.
	for range 4 {
		p.do(left)
	}
	p.wantShown("|[**]bc[**]")
	p.typeText("z")
	p.want("page(paragraph(z<bc>))")
}

// Backspace crosses a sign's glyph rather than delete through it; inside
// the sign it steps the mark down, both signs at once. With nothing held
// whole, the mark is not shown first.
func TestBackspaceCrossesASignAndStepsItInside(t *testing.T) {
	p := openWith(t, policy.Values{policy.EnabledKey("deletion"): false})
	p.typeText("a **b**")
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	p.do(back)
	p.wantShown("a [**]b[*|*]")
	p.want("page(paragraph(a <b>))")
	p.do(back)
	p.wantShown("a [*]b|[*]")
	if !hasMark(p, profile.Emphasis) || hasMark(p, profile.Strong) {
		t.Fatalf("strong did not step down to emphasis: %s", outline(p.e.Snapshot().Root()))
	}
	p.do(back)
	p.want("page(paragraph(a ))")
}

// Deleting next to a sign keeps the caret on its side of it: the text
// after a mark is after its closing sign, the text inside before it.
func TestDeletingKeepsTheCaretsSideOfASign(t *testing.T) {
	p := open(t)
	p.typeText("a *bc* d")
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	p.do(back)
	p.do(back)
	p.wantShown("a [*]bc[*]|")
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	p.do(left)
	p.do(back)
	p.wantShown("a [*]b|[*]")
}

func TestBackspaceInsideAGhostStepsIt(t *testing.T) {
	p := open(t)
	p.typeText("**b** c")
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	for range 3 {
		p.do(left)
	}
	p.wantShown("[**]b[*|*] c")
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if !hasMark(p, profile.Emphasis) || hasMark(p, profile.Strong) {
		t.Fatalf("strong did not step down: %s", outline(p.e.Snapshot().Root()))
	}
}

func TestHeadingMarkerSteps(t *testing.T) {
	p := open(t)
	p.typeText("## ")
	p.wantShown("[## ]|")
	p.typeText("Head")
	p.wantShown("[## ]Head|")
	for range 4 {
		p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	}
	p.wantShown("[## ]|Head")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.wantShown("[##| ]Head")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.wantShown("[#|# ]Head")
	// A "#" typed inside the marker makes the heading one level deeper.
	p.typeText("#")
	if n := p.e.Snapshot().Root().Slot(0).Child(0); profile.HeadingLevel(n) != 3 {
		t.Fatalf("level %d after typing #", profile.HeadingLevel(n))
	}
	p.want("page(heading(Head))")
	// The caret keeps its place from the text, after the glyph typed.
	p.wantShown("[##|# ]Head")
	// Backspace at the text's start steps it back up, then out.
	for range 2 {
		p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	}
	p.wantShown("[### ]|Head")
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	for _, level := range []int{2, 1} {
		p.do(back)
		if n := p.e.Snapshot().Root().Slot(0).Child(0); profile.HeadingLevel(n) != level {
			t.Fatalf("level %d, want %d", profile.HeadingLevel(n), level)
		}
	}
	p.wantShown("[# ]|Head")
	p.do(back)
	p.want("page(paragraph(Head))")
	p.wantShown("|Head")
}

// Each place a heading's marker holds edits it: a "#" typed before or
// among its glyphs is one level deeper, and deleting a glyph from either
// side one level shallower. Its separator is a gap before the text.
func TestEveryMarkerPlaceEditsTheMarker(t *testing.T) {
	typed := vocab.Type{Text: "#"}
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	fwd := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
	for _, tc := range []struct {
		at    string
		do    vocab.Intent
		level int
		shown string
	}{
		{"[## ]|Head", typed, 2, "[## ]#|Head"},
		{"[##| ]Head", typed, 3, "[###| ]Head"},
		{"[#|# ]Head", typed, 3, "[##|# ]Head"},
		{"|[## ]Head", typed, 3, "[#|## ]Head"},
		{"[## ]|Head", back, 1, "[# ]|Head"},
		{"[##| ]Head", back, 1, "[#| ]Head"},
		{"[#|# ]Head", back, 1, "|[# ]Head"},
		{"[##| ]Head", fwd, 2, "[## ]|Head"},
		{"[#|# ]Head", fwd, 1, "[#| ]Head"},
		{"|[## ]Head", fwd, 1, "|[# ]Head"},
	} {
		p := open(t)
		p.typeText("## Head")
		for p.shown() != tc.at {
			p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
		}
		p.do(tc.do)
		if n := p.e.Snapshot().Root().Slot(0).Child(0); profile.HeadingLevel(n) != tc.level {
			t.Fatalf("%v at %s: level %d, want %d", tc.do, tc.at, profile.HeadingLevel(n), tc.level)
		}
		if got := p.shown(); got != tc.shown {
			t.Fatalf("%v at %s: shown %q, want %q", tc.do, tc.at, got, tc.shown)
		}
	}
	// An item's marker takes no glyph: "#" is its text's.
	for _, at := range []string{"[-| ]a", "|[- ]a"} {
		p := open(t)
		p.typeText("- a")
		for p.shown() != at {
			p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
		}
		p.do(typed)
		p.want("page(list(item(#a|)))")
	}
}

// How notation draws never changes how it edits: the same keys make the
// same page, with the caret at the same place, whether signs are shown or
// hidden and whichever spelling they are drawn in.
func TestNotationEditsAlikeHoweverDrawn(t *testing.T) {
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	scripts := map[string]func(p *page){
		"close with *":    func(p *page) { p.typeText("a *em*x") },
		"close with _":    func(p *page) { p.typeText("a _em_x") },
		"strong":          func(p *page) { p.typeText("a **b** c") },
		"leave":           func(p *page) { p.typeText("a *em"); p.do(vocab.Leave{}); p.typeText("x") },
		"backspace after": func(p *page) { p.typeText("a *em* b"); p.do(left); p.do(left); p.do(back) },
		"backspace twice": func(p *page) { p.typeText("a *em* b"); p.do(left); p.do(left); p.do(back); p.do(back) },
		"step up at start": func(p *page) {
			p.typeText("a *em*")
			for range 3 {
				p.do(left)
			}
			p.typeText("*")
		},
		"type at the end": func(p *page) {
			p.typeText("a *em* b")
			for range 3 {
				p.do(left)
			}
			p.typeText("_")
		},
	}
	modes := []policy.Values{
		{notation.ID + ".reveal": notation.RevealHidden},
		{notation.ID + ".underscores": true},
		{notation.ID + ".reveal": notation.RevealHidden, notation.ID + ".underscores": true},
	}
	for name, script := range scripts {
		want := open(t)
		script(want)
		for _, values := range modes {
			p := openWith(t, values)
			script(p)
			if got, w := outline(p.e.Snapshot().Root()), outline(want.e.Snapshot().Root()); got != w {
				t.Errorf("%s under %v: %s, shown makes %s", name, values, got, w)
			}
			if got, w := p.head(), want.head(); got != w {
				t.Errorf("%s under %v: caret at %v, shown at %v", name, values, got, w)
			}
		}
	}
}

func TestFormatAtAGhostStandsUntilTyped(t *testing.T) {
	p := open(t)
	p.typeText("**b**")
	p.wantShown("[**]b[**]|")
	// Bold is off after the closing ghost; toggling it on stands.
	p.do(profile.Format{Mark: profile.Strong})
	p.typeText("c")
	p.want("page(paragraph(<bc>))")
}

func hasMark(p *page, k doc.MarkKind) bool {
	found := false
	p.e.Snapshot().Root().Walk(func(n *doc.Node) bool {
		for _, s := range n.Slots() {
			if s.IsInline() {
				s.Content().Each(func(_ int, it doc.Item) bool {
					if it.Marks.Has(k) {
						found = true
					}
					return true
				})
			}
		}
		return true
	})
	return found
}

// A mark pairs as it is typed: content after its opener opens it with the
// closing sign ahead, its character typed at the closing sign steps over
// it, and typed after the opening sign spells it up, both signs at once.
func TestMarksPairAsTyped(t *testing.T) {
	p := open(t)
	p.typeText("a *s")
	p.wantShown("a [*]s|[*]")
	p.typeText("*")
	p.wantShown("a [*]s[*]|")

	p = open(t)
	p.typeText("a **s*")
	p.wantShown("a [**]s[*|*]")
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.wantShown("a [*]s|[*]")

	p = open(t)
	p.typeText("a *s")
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.typeText("*")
	p.wantShown("a [**]|s[**]")

	p = open(t)
	p.typeText("5 * 3, 5*3 and `x")
	p.wantShown("5 * 3, 5*3 and [`]x|[`]")
	p.typeText("yz`")
	p.wantShown("5 * 3, 5*3 and [`]xyz[`]|")
}

// An edit beside a sign carries the caret by the content that survives
// next to it: deleting the character between the caret and a sign keeps
// the caret on its side of the sign.
func TestDeletingBesideASignKeepsTheCaretsSide(t *testing.T) {
	left := vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	fwd := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
	for _, c := range []struct {
		typed, from string
		in          vocab.Intent
		want        string
	}{
		{"a *xbc", "a [*]x|bc[*]", back, "a [*]|bc[*]"},
		{"a *bcx", "a [*]bcx|[*]", back, "a [*]bc|[*]"},
		{"a *bc*x", "a [*]bc[*]x|", back, "a [*]bc[*]|"},
		{"a *xbc", "a [*]|xbc[*]", fwd, "a [*]|bc[*]"},
		{"a *bcx", "a [*]bc|x[*]", fwd, "a [*]bc|[*]"},
		{"a *bc*x", "a [*]bc[*]|x", fwd, "a [*]bc[*]|"},
	} {
		p := open(t)
		p.typeText(c.typed)
		for i := 0; i < 20 && p.shown() != c.from; i++ {
			p.do(left)
		}
		p.wantShown(c.from)
		p.do(c.in)
		p.wantShown(c.want)
	}
}

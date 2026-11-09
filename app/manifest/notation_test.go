package manifest

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
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

func TestGhostsAreRevealedOnlyNearTheCaret(t *testing.T) {
	p := open(t)
	p.typeText("a *em*")
	p.do(vocab.Break{})
	p.typeText("next")
	// The caret is in the second paragraph: the first shows no ghosts.
	first := p.e.Snapshot().Root().Slot(0).Child(0).ID()
	for _, it := range p.e.Present(1, []doc.ID{first})[0].Surfaces[0].Flow.Items {
		if it.Kind == present.GhostItem {
			t.Fatalf("a ghost far from the caret: %q", it.Spelling)
		}
	}
	p.e.SetPresentOptions(1, present.Options{Reveal: present.RevealShown})
	ghosts := 0
	for _, it := range p.e.Present(1, []doc.ID{first})[0].Surfaces[0].Flow.Items {
		if it.Kind == present.GhostItem {
			ghosts++
		}
	}
	if ghosts != 2 {
		t.Fatalf("%d ghosts shown, want 2", ghosts)
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

func TestBackspaceStepsAGhostDown(t *testing.T) {
	p := open(t)
	p.typeText("a **b**")
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	p.do(back)
	p.wantShown("a [*]b[*]|")
	if !hasMark(p, vocab.Emphasis) || hasMark(p, vocab.Strong) {
		t.Fatalf("strong did not step down to emphasis: %s", outline(p.e.Snapshot().Root()))
	}
	p.do(back)
	p.want("page(paragraph(a b))")
	p.wantShown("a b|")
	p.do(back)
	p.want("page(paragraph(a ))")
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
	if !hasMark(p, vocab.Emphasis) || hasMark(p, vocab.Strong) {
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
	p.wantShown("[#|# ]Head")
	// A "#" typed inside the marker makes the heading one level deeper.
	p.typeText("#")
	if n := p.e.Snapshot().Root().Slot(0).Child(0); vocab.HeadingLevel(n) != 3 {
		t.Fatalf("level %d after typing #", vocab.HeadingLevel(n))
	}
	p.want("page(heading(Head))")
	// The caret keeps its place from the text, after the glyph typed.
	p.wantShown("[##|# ]Head")
	// Backspace at the text's start steps it back up, then out.
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	p.wantShown("[### ]|Head")
	back := vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	for _, level := range []int{2, 1} {
		p.do(back)
		if n := p.e.Snapshot().Root().Slot(0).Child(0); vocab.HeadingLevel(n) != level {
			t.Fatalf("level %d, want %d", vocab.HeadingLevel(n), level)
		}
	}
	p.wantShown("[# ]|Head")
	p.do(back)
	p.want("page(paragraph(Head))")
	p.wantShown("|Head")
}

func TestHiddenNotationKeepsOneStopPerPosition(t *testing.T) {
	p := open(t)
	p.e.SetPresentOptions(1, present.Options{Reveal: present.RevealHidden})
	p.typeText("a **b**")
	p.wantShown("a b|")
	if h := p.head(); h.Stop != 0 {
		t.Fatalf("a hidden ghost stop %d", h.Stop)
	}
	// Without ghosts, Backspace deletes characters.
	p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.want("page(paragraph(a ))")
}

func TestFormatAtAGhostStandsUntilTyped(t *testing.T) {
	p := open(t)
	p.typeText("**b**")
	p.wantShown("[**]b[**]|")
	// Bold is off after the closing ghost; toggling it on stands.
	p.do(vocab.Format{Mark: vocab.Strong})
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

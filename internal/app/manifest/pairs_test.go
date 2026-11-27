package manifest

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func TestAnOpenerClosesItselfAndItsCloserIsSteppedOver(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.want("page(paragraph(a (b)))")
	p.typeText(")c")
	p.want("page(paragraph(a (b)c))")
	// Before a word, nothing is typed ahead.
	p = open(t)
	p.typeText("x")
	p.do(left)
	p.typeText("(")
	p.want("page(paragraph((x))")
}

func TestQuotesPairButCloseAfterAWord(t *testing.T) {
	p := open(t)
	p.typeText(`say "hi`)
	p.want(`page(paragraph(say "hi"))`)
	p.typeText(`" ok`)
	p.want(`page(paragraph(say "hi" ok))`)
}

// A caret inside a pair or a marked run paints the whole scope.
func TestPairsAndMarksPaintWhileTheCaretIsInside(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	if p.painted(vocab.GroupPairMatch) != 1 {
		t.Fatalf("pair highlights = %d", p.painted(vocab.GroupPairMatch))
	}
	p.do(vocab.Leave{})
	if p.painted(vocab.GroupPairMatch) != 0 {
		t.Fatalf("a pair outside the caret is painted %d times", p.painted(vocab.GroupPairMatch))
	}

	p = open(t)
	p.typeText("a *b")
	if p.painted(vocab.GroupPairMatch) != 1 {
		t.Fatalf("mark highlights = %d", p.painted(vocab.GroupPairMatch))
	}
}

func (p *page) armed() int {
	n := 0
	for _, h := range p.e.Decorations(1).Paints {
		if h.Group == vocab.GroupArmed {
			n++
		}
	}
	return n
}

func (p *page) painted(group vocab.Group) int {
	n := 0
	for _, h := range p.e.Decorations(1).Paints {
		if h.Group == group {
			n++
		}
	}
	return n
}

func (p *page) guideCount(command string) int {
	n := 0
	for _, g := range p.e.Decorations(1).Guides {
		if string(g.Command) == command {
			n++
		}
	}
	return n
}

func (p *page) guideAt(command string) (doc.Caret, bool) {
	for _, g := range p.e.Decorations(1).Guides {
		if string(g.Command) == command {
			return g.At, true
		}
	}
	return doc.Caret{}, false
}

// Deleting into a pair from either side arms both sides; the next press
// deletes both and keeps what they held. Anything else disarms.
func TestDeletingIntoAPairArmsBothSides(t *testing.T) {
	p := open(t)
	p.typeText("a [")
	p.do(backspace)
	p.want("page(paragraph(a []))")
	if p.armed() != 2 {
		t.Fatalf("%d sides armed, want 2", p.armed())
	}
	p.do(backspace)
	p.want("page(paragraph(a ))")

	p = open(t)
	p.typeText("a (bc")
	p.do(right)
	p.do(backspace)
	p.want("page(paragraph(a (bc)))")
	p.do(backspace)
	p.want("page(paragraph(a bc))")

	p = open(t)
	p.typeText("(bc")
	p.do(left)
	p.do(left)
	p.do(backspace)
	p.typeText("x")
	if p.armed() != 0 {
		t.Fatal("typing kept the pair armed")
	}
	p.want("page(paragraph((xbc)))")
}

// A mark between its signs is a pair too: Backspace after its opening
// sign arms both signs, and the next press unmarks the run.
func TestDeletingIntoAMarkArmsItsSigns(t *testing.T) {
	p := open(t)
	p.typeText("a *bc*")
	p.want("page(paragraph(a <bc>))")
	for range 3 {
		p.do(left)
	}
	p.wantShown("a [*]|bc[*]")
	p.do(backspace)
	if p.armed() != 2 {
		t.Fatalf("%d signs armed, want 2", p.armed())
	}
	p.do(backspace)
	p.want("page(paragraph(a bc))")
}

// From outside a mark, Backspace arms its signs as it arms a bracket's.
func TestDeletingIntoAMarkFromOutsideArmsItsSigns(t *testing.T) {
	p := open(t)
	p.typeText("a **bc** d")
	p.do(backspace)
	p.do(backspace)
	p.wantShown("a [**]bc[**]|")
	p.do(backspace)
	if p.armed() != 2 {
		t.Fatalf("%d signs armed, want 2", p.armed())
	}
	p.wantShown("a [**]bc[**]|")
	p.do(backspace)
	p.want("page(paragraph(a bc))")
}

// The way back is offered right after the scope, not inside it, so moving
// inside and out again finds it there.
func TestTheWayBackFromAMarkStandsAtItsExit(t *testing.T) {
	p := open(t)
	p.typeText("a *bc")
	p.do(vocab.Leave{})
	p.do(left)
	p.wantShown("a [*]bc|[*]")
	if p.guideCount("core.leave") != 1 || p.guideCount("core.return") != 0 {
		t.Fatalf("inside, guides %+v", p.e.Decorations(1).Guides)
	}
	p.do(right)
	if p.guideCount("core.return") != 1 || p.guideCount("core.leave") != 0 {
		t.Fatalf("back at the exit, guides %+v", p.e.Decorations(1).Guides)
	}
	p.do(vocab.Return{})
	p.wantShown("a [*]bc|[*]")
}

// Leave exits a mark past its closing sign, where typing is unmarked.
func TestLeaveExitsAMark(t *testing.T) {
	p := open(t)
	p.typeText("a *bc*")
	p.do(left)
	p.wantShown("a [*]bc|[*]")
	p.do(vocab.Leave{})
	p.typeText("d")
	p.want("page(paragraph(a <bc>d))")
}

func TestTypingAnOpenerWrapsTheSelection(t *testing.T) {
	p := open(t)
	p.typeText("one two")
	for range 3 {
		p.do(extend(left))
	}
	p.typeText("(")
	p.want("page(paragraph(one (two)))")
	p.typeText("2")
	p.want("page(paragraph(one (2)))")
}

func TestLeaveExitsAPairAndReturnGoesBack(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.Leave{})
	p.typeText("!")
	p.want("page(paragraph(a (b)!))")
	p = open(t)
	p.typeText("a (b")
	p.do(vocab.Leave{})
	p.do(vocab.Return{})
	p.typeText("c")
	p.want("page(paragraph(a (bc)))")
}

// Outside every scope Leave and Return do nothing, so their key falls
// through to the next command it is bound to.
func TestLeaveOutsideAScopeIsUnavailable(t *testing.T) {
	p := open(t)
	p.typeText("plain")
	out, err := p.e.Preview(1, vocab.Input{Intent: vocab.Leave{}, At: p.now})
	if err != nil || out.Available {
		t.Fatalf("leave available outside a scope: %+v %v", out, err)
	}
	out, _ = p.e.Preview(1, vocab.Input{Intent: vocab.Return{}, At: p.now})
	if out.Available {
		t.Fatal("return available with nowhere to go")
	}
}

func TestGuidesShowWhereLeaveLands(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	at, ok := p.guideAt("core.leave")
	if !ok || p.guideCount("core.leave") != 1 {
		t.Fatalf("guides %+v", p.e.Decorations(1).Guides)
	}
	if at.Pos.Offset != 5 {
		t.Fatalf("leave lands at %d, want after the closer", at)
	}
	p.do(vocab.Leave{})
	if p.guideCount("core.return") != 1 || p.guideCount("core.leave") != 0 {
		t.Fatalf("after leaving, guides %+v", p.e.Decorations(1).Guides)
	}
}

// Return is offered only right after a scope: writing on moves away.
func TestReturnStandsOnlyRightAfterAScope(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.Leave{})
	p.typeText(" c")
	if p.guideCount("core.return") != 0 {
		t.Fatalf("guides %+v", p.e.Decorations(1).Guides)
	}
	p.do(vocab.Return{})
	p.typeText("!")
	p.want("page(paragraph(a (b) c!))")
}

// The way back shows where it lands: inside the pair left.
func TestTheReturnGuideShowsWhereItLands(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	inside := p.head()
	p.do(vocab.Leave{})
	if at, ok := p.guideAt("core.return"); !ok || at != inside || p.guideCount("core.return") != 1 {
		t.Fatalf("guides %+v, want return at %v", p.e.Decorations(1).Guides, inside)
	}
}

// Moving away and back to a scope's end offers the way back again.
func TestTheWayBackOutlastsMovingAround(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.Leave{})
	p.do(left)
	p.do(left)
	if p.guideCount("core.leave") != 1 || p.guideCount("core.return") != 0 {
		t.Fatalf("inside, guides %+v", p.e.Decorations(1).Guides)
	}
	p.do(right)
	p.do(right)
	if p.guideCount("core.return") != 1 || p.guideCount("core.leave") != 0 {
		t.Fatalf("back at the exit, guides %+v", p.e.Decorations(1).Guides)
	}
}

// Return reads where the caret is, not how it got there: right after a
// scope, by whichever route, it enters the scope before its end.
func TestReturnEntersTheScopeBehindTheCaretByAnyRoute(t *testing.T) {
	routes := []struct {
		name   string
		reach  func(p *page)
		inside string
	}{
		{"leave", func(p *page) { p.typeText("a (bc"); p.do(vocab.Leave{}) }, "a (bc|)"},
		{"type the closer", func(p *page) { p.typeText("a (bc)") }, "a (bc|)"},
		{"write on, come back", func(p *page) {
			p.typeText("a (bc")
			p.do(vocab.Leave{})
			p.typeText(" x")
			p.do(left)
			p.do(left)
		}, "a (bc|) x"},
		{"a mark's closer", func(p *page) { p.typeText("a *bc*") }, "a [*]bc|[*]"},
		{"a mark left", func(p *page) { p.typeText("a *bc"); p.do(vocab.Leave{}) }, "a [*]bc|[*]"},
	}
	for _, r := range routes {
		p := open(t)
		r.reach(p)
		if p.guideCount("core.return") != 1 {
			t.Fatalf("%s: guides %+v", r.name, p.e.Decorations(1).Guides)
		}
		p.do(vocab.Return{})
		p.wantShown(r.inside)
	}
}

// A closer typed at a pair's end steps over the one there, however that
// one was typed.
func TestAHandTypedCloserIsSteppedOver(t *testing.T) {
	p := openWith(t, policy.Values{"pairs.autoClose": false})
	p.typeText("a (bc)")
	p.do(left)
	p.typeText(")")
	p.want("page(paragraph(a (bc)))")
	p.wantShown("a (bc)|")
}

func TestPairsRespectMarkBoundariesAndWrapping(t *testing.T) {
	// Typing ( right after a closed mark stays unmarked outside it.
	p := open(t)
	p.typeText("a *bc*(")
	p.wantShown("a [*]bc[*](|)")

	// Typing ( right after an opening mark sign (before a non-word) stays marked inside it.
	p = open(t)
	p.typeText("a *bc*")
	for i := 0; i < 10 && p.shown() != "a [*]|bc[*]"; i++ {
		p.do(left)
	}
	p.typeText(" ")
	p.do(left)
	p.wantShown("a [*]| bc[*]")
	p.typeText("(")
	p.wantShown("a [*](|) bc[*]")

	// Wrapping |*bc*| from outside the signs keeps both parens unmarked.
	p = open(t)
	p.typeText("a *bc*")
	for range 4 {
		p.do(extend(left))
	}
	p.typeText("(")
	p.want("page(paragraph(a (<bc>)))")

	// Wrapping *|bc|* inside the signs marks both parens with the content.
	p = open(t)
	p.typeText("a *bc*")
	p.do(left)
	for range 2 {
		p.do(extend(left))
	}
	p.typeText("(")
	p.want("page(paragraph(a <(bc)>))")
}

// In code a pair is the language's: it closes and is stepped over as
// typed, but Tab, Leave's key, nests the line rather than leaving it.
func TestAPairInCodeIsNoScopeToLeave(t *testing.T) {
	p := open(t)
	p.typeText("```go")
	p.do(vocab.Break{})
	p.typeText("f(\"x")
	p.want("page(code(f(\"x\")))")
	out, err := p.e.Preview(1, vocab.Input{Intent: vocab.Leave{}, At: p.now})
	if err != nil || out.Available {
		t.Fatalf("leave available in code: %+v %v", out, err)
	}
	if p.guideCount("core.leave") != 0 || p.guideCount("core.return") != 0 {
		t.Fatalf("guides in code %+v", p.e.Decorations(1).Guides)
	}
	p.typeText("\")")
	p.want("page(code(f(\"x\")))")
	// Nor does a guide show where Tab would indent to, or a wash the
	// pair's inside: an indent is an edit, and code's lines are no scope.
	p.do(vocab.Break{})
	p.typeText("    x")
	if d := p.e.Decorations(1); len(d.Guides) != 0 || len(d.Paints) != 0 {
		t.Fatalf("decorations in code %+v %+v", d.Guides, d.Paints)
	}
}

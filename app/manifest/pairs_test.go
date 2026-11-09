package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
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

// Deleting into a pair from either side selects it whole; the next press
// deletes it, both sides at once.
func TestDeletingIntoAPairSelectsItFirst(t *testing.T) {
	p := open(t)
	p.typeText("a [")
	p.do(backspace)
	p.want("page(paragraph(a []))")
	p.do(backspace)
	p.want("page(paragraph(a ))")

	p = open(t)
	p.typeText("a (bc")
	p.do(right)
	p.do(backspace)
	p.want("page(paragraph(a (bc)))")
	p.do(backspace)
	p.want("page(paragraph(a ))")

	p = open(t)
	p.typeText("(bc")
	p.do(left)
	p.do(left)
	p.do(backspace)
	p.want("page(paragraph((bc)))")
	p.typeText("x")
	p.want("page(paragraph(x))")
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
	guides := p.e.Decorations(1).Guides
	if len(guides) != 1 || guides[0].Command != "core.leave" {
		t.Fatalf("guides %+v", guides)
	}
	if at := guides[0].At.Pos.Offset; at != 5 {
		t.Fatalf("leave lands at %d, want after the closer", at)
	}
	p.do(vocab.Leave{})
	guides = p.e.Decorations(1).Guides
	if len(guides) != 1 || guides[0].Command != "core.return" {
		t.Fatalf("after leaving, guides %+v", guides)
	}
}

// A press away from the last edit offers Return back to it, for a while.
func TestReturnToEditAfterAPress(t *testing.T) {
	p := open(t)
	p.typeText("first")
	p.do(vocab.Break{})
	p.typeText("second")
	first := p.e.Snapshot().Root().Slot(0).Child(0)
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: first.ID()}, 2)), Clicks: 1})
	p.do(vocab.Return{})
	p.typeText("!")
	p.want("page(paragraph(first) paragraph(second!))")
}

// Writing on after leaving forgets the way back.
func TestWritingOnForgetsTheScopeLeft(t *testing.T) {
	p := open(t)
	p.typeText("a (b")
	p.do(vocab.Leave{})
	p.typeText(" c")
	if g := p.e.Decorations(1).Guides; len(g) != 0 {
		t.Fatalf("guides %+v", g)
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
	g := p.e.Decorations(1).Guides
	if len(g) != 1 || g[0].Command != "core.return" || g[0].At != inside {
		t.Fatalf("guides %+v, want return at %v", g, inside)
	}
}

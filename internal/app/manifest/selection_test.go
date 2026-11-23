package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/policies/notation"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

var (
	left      = vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	right     = vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
	backspace = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	forward   = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
)

func extend(m vocab.Move) vocab.Move { m.Extend = true; return m }

// selectPage grows SelectScope to the outermost scope, as the scopes
// policy intends.
func (p *page) selectPage() {
	for range 8 {
		p.do(vocab.SelectScope{})
	}
}

// What is shown selected is what an edit of the selection takes: a
// selection from before a block's marker holds the block's structure.

func TestSelectAllDeletesAHeadingWhole(t *testing.T) {
	p := open(t)
	p.typeText("## Head")
	p.do(vocab.SelectScope{})
	p.do(backspace)
	p.want("page(paragraph())")
	p.wantShown("|")
}

func TestTypingOverAllLeavesOneParagraph(t *testing.T) {
	p := open(t)
	p.typeText("# Title")
	p.do(vocab.Break{})
	p.typeText("body")
	p.selectPage()
	p.typeText("x")
	p.want("page(paragraph(x))")
}

func TestASelectionAfterTheMarkerKeepsTheBlock(t *testing.T) {
	p := open(t)
	p.typeText("# Head")
	for range 4 {
		p.do(extend(left))
	}
	p.wantShown("[# ]|Head")
	p.do(backspace)
	p.want("page(heading())")
	p.wantShown("[# ]|")
}

func TestARangeFromBeforeAMarkerUnmarksWhereItEnds(t *testing.T) {
	p := open(t)
	p.typeText("# a")
	p.do(vocab.Break{})
	p.typeText("## bcd")
	p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
	p.wantShown("|[# ]a")
	for range 8 {
		p.do(extend(right))
	}
	p.do(backspace)
	p.want("page(paragraph(cd))")
}

func TestEnterAtAHeadingsStartOpensAParagraphBeforeIt(t *testing.T) {
	p := open(t)
	p.typeText("# Head")
	for range 4 {
		p.do(left)
	}
	p.wantShown("[# ]|Head")
	p.do(vocab.Break{})
	p.want("page(paragraph() heading(Head))")
	p.wantShown("[# ]|Head")
}

func TestBackspaceBeforeAMarkerJoinsTheBlockBefore(t *testing.T) {
	p := open(t)
	p.typeText("p")
	p.do(vocab.Break{})
	p.typeText("# Head")
	for range 6 {
		p.do(left)
	}
	p.wantShown("|[# ]Head")
	p.do(backspace)
	p.want("page(paragraph(pHead))")
}

func TestDeleteBeforeAMarkerStepsItDown(t *testing.T) {
	p := open(t)
	p.typeText("## Head")
	for range 7 {
		p.do(left)
	}
	p.wantShown("|[## ]Head")
	p.do(forward)
	if n := p.e.Snapshot().Root().Slot(0).Child(0); profile.HeadingLevel(n) != 1 {
		t.Fatalf("level %d after deleting into the marker", profile.HeadingLevel(n))
	}
}

func TestBackspaceAtAHiddenHeadingsStartStepsItDown(t *testing.T) {
	p := revealed(t, notation.RevealHidden)
	p.typeText("## Head")
	for range 4 {
		p.do(left)
	}
	p.do(backspace)
	if n := p.e.Snapshot().Root().Slot(0).Child(0); profile.HeadingLevel(n) != 1 {
		t.Fatalf("level %d; hidden notation steps like shown notation", profile.HeadingLevel(n))
	}
}

// The engine publishes what the selection covers: the renderer paints
// only that.
func TestSelectAllCoversTheHeadingWhole(t *testing.T) {
	p := open(t)
	p.typeText("## Head")
	p.do(vocab.SelectScope{})
	covers := p.e.Decorations(1).Paints[0].Covers
	if len(covers) != 1 || !covers[0].Whole || len(covers[0].Spans) != 1 {
		t.Fatalf("covers %+v; select-all holds the heading whole", covers)
	}
}

func TestASelectionAfterTheMarkerCoversOnlyText(t *testing.T) {
	p := open(t)
	p.typeText("# Head")
	for range 4 {
		p.do(extend(left))
	}
	covers := p.e.Decorations(1).Paints[0].Covers
	if len(covers) != 1 || covers[0].Whole || len(covers[0].Spans) != 1 {
		t.Fatalf("covers %+v; the text alone is covered", covers)
	}
	if sp := covers[0].Spans[0]; sp.To-sp.From != 4 {
		t.Fatalf("span %+v covers %d steps, want 4", sp, sp.To-sp.From)
	}
}

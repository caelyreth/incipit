package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

var (
	left      = vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	right     = vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
	backspace = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}
	forward   = vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Forward}
)

func extend(m vocab.Move) vocab.Move { m.Extend = true; return m }

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
	p.do(vocab.SelectScope{})
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
	for range 6 {
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
	for range 5 {
		p.do(left)
	}
	p.wantShown("|[# ]Head")
	p.do(backspace)
	p.want("page(paragraph(pHead))")
}

func TestDeleteBeforeAMarkerStepsItDown(t *testing.T) {
	p := open(t)
	p.typeText("## Head")
	for range 6 {
		p.do(left)
	}
	p.wantShown("|[## ]Head")
	p.do(forward)
	if n := p.e.Snapshot().Root().Slot(0).Child(0); vocab.HeadingLevel(n) != 1 {
		t.Fatalf("level %d after deleting into the marker", vocab.HeadingLevel(n))
	}
}

func TestBackspaceAtAHiddenHeadingsStartStepsItDown(t *testing.T) {
	p := open(t)
	p.e.SetPresentOptions(1, hidden())
	p.typeText("## Head")
	for range 4 {
		p.do(left)
	}
	p.do(backspace)
	if n := p.e.Snapshot().Root().Slot(0).Child(0); vocab.HeadingLevel(n) != 1 {
		t.Fatalf("level %d; hidden notation steps like shown notation", vocab.HeadingLevel(n))
	}
}

func hidden() present.Options { return present.Options{Reveal: present.RevealHidden} }

package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"

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

// Text typed over a range from before an item's marker lands in a block
// the gap left takes: a new item where the range starts inside a list,
// a paragraph where that list went whole, the caret after it (seeds 52
// and 280 of TestRandomShapesKeepTheInvariants, rejected before).
func TestTypingOverARangeFromBeforeAMarkerKeepsItsLists(t *testing.T) {
	item := func(text string, body ...*doc.Node) *doc.Node {
		return doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem(text, nil)), doc.BlockSlot(body...))
	}
	list := func(items ...*doc.Node) *doc.Node {
		return doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(items...))
	}
	head := func(n *doc.Node) doc.Caret { return doc.Caret{Pos: doc.Pos{Node: n.ID()}, Stop: doc.StopFirst} }
	cases := []struct {
		first []*doc.Node
		from  int
		over  vocab.Intent
		want  string
	}{
		{[]*doc.Node{item("a")}, 0, vocab.Type{Text: "x"}, "page(paragraph(xy) list(item(c|)) paragraph())"},
		{[]*doc.Node{item("a"), item("a2")}, 1, vocab.Type{Text: "x"}, "page(list(item(a|) item(xy|)) list(item(c|)) paragraph())"},
		// Enter opens the block alike (seed 957).
		{[]*doc.Node{item("a")}, 0, vocab.Break{}, "page(paragraph(y) list(item(c|)) paragraph())"},
	}
	for _, c := range cases {
		p := open(t)
		tx := p.e.Snapshot().Begin(doc.Origin{Class: doc.OriginCommand})
		blocks := []*doc.Node{list(c.first...), list(item("b", list(item("c"))))}
		if err := tx.Apply(doc.Insert{Parent: 1, Nodes: blocks}); err != nil {
			t.Fatal(err)
		}
		snap, err := tx.Commit()
		if err != nil {
			t.Fatal(err)
		}
		prog, _ := Compile(nil, true)
		p.e = engine.New(snap, prog, engine.Options{})
		p.e.OpenView(1)
		root := snap.Root()
		from := head(root.Slot(0).Child(0).Slot(0).Child(c.from))
		nested := root.Slot(0).Child(1).Slot(0).Child(0).Slot(1).Child(0).Slot(0).Child(0)
		p.do(vocab.Place{Stop: head(nested), Clicks: 1, Extend: true, Anchor: &from})
		p.do(c.over)
		p.do(vocab.Type{Text: "y"})
		p.want(c.want)
	}
}

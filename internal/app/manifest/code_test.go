package manifest

import (
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// openCode opens a page on a code block in go, holding text.
func openCode(t *testing.T, text string) *page {
	t.Helper()
	p := open(t)
	p.typeText("```go")
	p.do(vocab.Break{})
	p.do(vocab.Paste{Text: text})
	return p
}

// SelectScope in code selects the code, then the block with its fences,
// whole, as a node: a delete takes it at once, as it takes a figure.
func TestSelectAllInCodeSelectsTheCodeThenTheBlock(t *testing.T) {
	p := openCode(t, "a\nb")
	for range 3 {
		p.do(vocab.Break{})
	}
	p.typeText("after")
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: p.e.Snapshot().Root().Slot(0).Child(0).ID()}, 1))})
	p.do(vocab.SelectScope{})
	from, to := p.selectedRange()
	if from.Offset != 0 || to.Offset != 3 || from.Node == p.e.Snapshot().Root().ID() {
		t.Fatalf("first select-all %v..%v, want the code", from, to)
	}
	if d := p.e.Decorations(1); len(d.Paints) == 0 || d.Paints[0].Covers[0].Whole {
		t.Fatalf("the code alone is not the block whole: %+v", d.Paints)
	}
	p.do(vocab.SelectScope{})
	if v, _ := p.e.View(1); v.Selection.Kind != doc.SelectNode {
		t.Fatalf("second select-all %+v, want the block as a node", v.Selection)
	}
	if covers := p.e.Decorations(1).Paints[0].Covers; len(covers) != 1 || !covers[0].Whole {
		t.Fatalf("second select-all covers %+v, want the block whole", covers)
	}
	p.do(vocab.Break{})
	p.want("page(code(a\nb) paragraph() paragraph(after))")
	p.do(vocab.Undo{})
	p.do(backspace)
	p.want("page(paragraph(after))")
}

// Backspace after a code block holds it first, whole, as after a figure;
// the next takes it.
func TestBackspaceAfterCodeHoldsItFirst(t *testing.T) {
	p := openCode(t, "x")
	for range 3 {
		p.do(vocab.Break{})
	}
	p.typeText("y")
	p.do(left)
	p.do(backspace)
	p.want("page(code(x) paragraph(y))")
	armed := false
	for _, pt := range p.e.Decorations(1).Paints {
		if pt.Group == vocab.GroupArmed && len(pt.Covers) == 1 && pt.Covers[0].Whole {
			armed = true
		}
	}
	if !armed {
		t.Fatalf("paints %+v, want the code held whole", p.e.Decorations(1).Paints)
	}
	p.do(backspace)
	p.want("page(paragraph(y))")
}

func (p *page) selectedRange() (doc.Pos, doc.Pos) {
	p.t.Helper()
	v, _ := p.e.View(1)
	return v.Selection.Anchor.Pos, v.Selection.Head.Pos
}

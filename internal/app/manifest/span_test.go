package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// span is what the view's selection spans.
func (p *page) span() policy.SelectionSpan {
	p.t.Helper()
	stops, _ := p.e.Stops(1)
	v, _ := p.e.View(1)
	return policy.SpanOf(p.e.Snapshot(), stops, v.Selection)
}

func (p *page) selectText(ref doc.SlotRef, from, to int) {
	p.t.Helper()
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, from))})
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, to)), Extend: true})
}

// A selection's span is recognised by the slot's own words and
// sentences: a word, a sentence, other text, the block's whole text.
func TestASpanIsRecognisedByItsText(t *testing.T) {
	p := open(t)
	p.typeText("Tide pools. Shore birds.")
	ref, _ := vocab.FirstTextSlot(p.e.Snapshot().Root())
	for _, c := range []struct {
		from, to int
		want     policy.SpanShape
	}{
		{5, 5, policy.SpanCaret},
		{5, 10, policy.SpanWord},
		{0, 11, policy.SpanSentence},
		{0, 7, policy.SpanText},
		{0, 24, policy.SpanBlocks},
	} {
		p.selectText(ref, c.from, c.to)
		if s := p.span(); s.Shape != c.want || len(s.Blocks) != 1 {
			t.Fatalf("%d..%d spans %v over %d blocks, want %v over one", c.from, c.to, s.Shape, len(s.Blocks), c.want)
		}
	}
}

// A range across a list's items spans each item as a block of its own,
// each in the list.
func TestASpanAcrossItemsHoldsEachItem(t *testing.T) {
	p := open(t)
	p.typeText("- a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.Break{})
	p.typeText("c")
	list := p.e.Snapshot().Root().Slot(0).Children()[0]
	items := list.Slot(0).Children()
	first := doc.SlotRef{Node: items[0].ID()}
	last := doc.SlotRef{Node: items[len(items)-1].ID()}
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(first, 0))})
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(last, 1)), Extend: true})
	s := p.span()
	if s.Shape != policy.SpanBlocks || len(s.Blocks) != 3 {
		t.Fatalf("Selecting the list spans %v over %d blocks, want blocks over three.", s.Shape, len(s.Blocks))
	}
	for _, b := range s.Blocks {
		if _, ok := policy.Within(p.e.Snapshot(), b, profile.List); !ok {
			t.Fatalf("Block %v is not in the list.", b.ID())
		}
	}
}

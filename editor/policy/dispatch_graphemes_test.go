package policy

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// holds evaluates a condition with the caret at a position.
func holds(r doc.Reader, cond Cond, at doc.Pos) bool {
	return cond.test(&Ctx{r: r, view: &View{Selection: doc.Collapsed(doc.CaretAt(at))}})
}

func TestCharClassClassifiesTheWholeGrapheme(t *testing.T) {
	snap, ref := testPage(t, "éx")
	at := doc.At(ref, len("é"))
	if !holds(snap, PrevChar(vocab.ClassWord), at) || holds(snap, PrevChar(vocab.ClassCJK), at) {
		t.Fatal("before combining letter: want its base letter's class")
	}
	if !holds(snap, NextChar(vocab.ClassWord), doc.At(ref, 0)) || holds(snap, NextChar(vocab.ClassCJK), doc.At(ref, 0)) {
		t.Fatal("after combining letter: want its base letter's class")
	}
}

func TestCharClassAtAnInlineNodeIsEmpty(t *testing.T) {
	marks := doc.NewInterner()
	empty := marks.Empty()
	formula := doc.NewNodeWithID(3, profile.Formula, doc.Attrs{}, "", doc.InlineSlot(doc.TextItem("x", empty)))
	para := doc.InlineSlot(doc.TextItem("a", empty), doc.NodeItem(formula), doc.TextItem("b", empty))
	page := doc.NewNodeWithID(1, profile.Page, doc.Attrs{}, "", doc.BlockSlot(
		doc.NewNodeWithID(2, profile.Paragraph, doc.Attrs{}, "a", para),
	))
	snap, err := doc.NewSnapshot(profile.Schema(), marks, page, 99)
	if err != nil {
		t.Fatal(err)
	}
	all := vocab.ClassWord | vocab.ClassSpace | vocab.ClassCJK | vocab.ClassDigit
	if holds(snap, NextChar(all), doc.At(doc.SlotRef{Node: 2}, 1)) {
		t.Fatal("an inline node has a class")
	}
}

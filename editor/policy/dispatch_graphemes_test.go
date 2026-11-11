package policy

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

func TestCharClassClassifiesTheWholeGrapheme(t *testing.T) {
	snap, ref := testPage(t, "e\u0301x")
	e := evaluator{c: &Ctx{r: snap}}
	at := doc.At(ref, len("e\u0301"))
	if got := e.charClass(at, true); got&vocab.ClassLatin == 0 || got&vocab.ClassWord == 0 {
		t.Fatalf("before combining letter = %b, want its base letter's class", got)
	}
	if got := e.charClass(doc.At(ref, 0), false); got&vocab.ClassLatin == 0 || got&vocab.ClassWord == 0 {
		t.Fatalf("after combining letter = %b, want its base letter's class", got)
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
	ref := doc.SlotRef{Node: 2}
	e := evaluator{c: &Ctx{r: snap}}
	if got := e.charClass(doc.At(ref, 1), false); got != 0 {
		t.Fatalf("class at an inline node = %b, want 0", got)
	}
}

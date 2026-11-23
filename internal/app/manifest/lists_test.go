package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// twoItems types a list of two items with the caret at the end of the
// second.
func twoItems(t *testing.T) *page {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.typeText("two")
	p.want("page(list(item(one|) item(two|)))")
	p.wantShown("[- ]two|")
	return p
}

func TestEnterInAnEmptyItemLeavesTheList(t *testing.T) {
	p := open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.want("page(list(item(one|) item(|)))")
	p.do(vocab.Break{})
	p.want("page(list(item(one|)) paragraph())")
	p.typeText("x")
	p.want("page(list(item(one|)) paragraph(x))")
}

// Enter over a range from before an item's marker through its head
// unlists the item, as deleting the range does, and breaks what is left:
// the text the range ended in, a paragraph now.
func TestEnterOverAnItemFromBeforeItsMarker(t *testing.T) {
	p := twoItems(t)
	item := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(1).ID()
	p.do(vocab.Place{Stop: doc.Caret{Pos: doc.Pos{Node: item}, Stop: doc.StopFirst}})
	p.do(vocab.Place{Stop: doc.CaretAt(doc.Pos{Node: item, Offset: 3}), Extend: true})
	if s := p.selection(); s.Anchor.Stop >= 0 {
		t.Fatalf("selection %+v, want it from before the marker", s)
	}
	p.do(vocab.Break{})
	p.want("page(list(item(one|)) paragraph() paragraph())")
	p.typeText("x")
	p.want("page(list(item(one|)) paragraph() paragraph(x))")
}

func TestBackspaceAtAnItemsStartLeavesTheList(t *testing.T) {
	p := twoItems(t)
	for range 3 {
		p.do(left)
	}
	p.wantShown("[- ]|two")
	p.do(backspace)
	p.want("page(list(item(one|)) paragraph(two))")
	p.wantShown("|two")
}

func TestBackspaceBeforeAnItemsMarkerJoinsTheItemBefore(t *testing.T) {
	p := twoItems(t)
	for range 5 {
		p.do(left)
	}
	p.wantShown("|[- ]two")
	p.do(backspace)
	p.want("page(list(item(onetwo|)))")
}

func TestNestingStepsAnItemsLevel(t *testing.T) {
	p := twoItems(t)
	p.do(profile.Nest{Dir: vocab.Forward})
	p.want("page(list(item(one|list(item(two|)))))")
	// Backspace at a nested item's start steps it out one level only.
	for range 3 {
		p.do(left)
	}
	p.do(backspace)
	p.want("page(list(item(one|) item(two|)))")
	p.do(profile.Nest{Dir: vocab.Forward})
	p.do(profile.Nest{Dir: vocab.Backward})
	p.want("page(list(item(one|) item(two|)))")
}

func TestOutdentKeepsTheItemsAfterInOrder(t *testing.T) {
	p := open(t)
	p.typeText("- a")
	for _, s := range []string{"b", "c"} {
		p.do(vocab.Break{})
		p.typeText(s)
		p.do(profile.Nest{Dir: vocab.Forward})
	}
	p.want("page(list(item(a|list(item(b|list(item(c|)))))))")
	p = open(t)
	p.typeText("- a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(profile.Nest{Dir: vocab.Forward})
	p.do(vocab.Break{})
	p.typeText("c")
	p.want("page(list(item(a|list(item(b|) item(c|)))))")
	// Outdenting b brings c along under it.
	for range 5 {
		p.do(left)
	}
	p.wantShown("[- ]|b")
	p.do(profile.Nest{Dir: vocab.Backward})
	p.want("page(list(item(a|) item(b|list(item(c|)))))")
}

func TestSelectAllDeletesAListWhole(t *testing.T) {
	p := twoItems(t)
	p.selectPage()
	p.do(backspace)
	p.do(backspace)
	p.want("page(paragraph())")
}

// Every item spells its marker, wherever the caret is: notation never
// restyles as the caret moves.
func TestEveryItemSpellsItsMarker(t *testing.T) {
	p := twoItems(t)
	units := p.e.Units(1)
	ids := []doc.ID{units[0].ID, units[1].ID}
	for i, pres := range p.e.Present(1, ids) {
		spelled := ""
		for _, it := range pres.Surfaces[0].Flow.Items {
			if it.Kind == present.GhostItem {
				spelled += string(it.Spelling)
			}
		}
		if spelled != "- " {
			t.Fatalf("item %d spells %q, want \"- \"", i, spelled)
		}
	}
}

func TestOrderedMarkersSpellTheirNumbers(t *testing.T) {
	p := open(t)
	p.typeText("3. a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.wantShown("[4. ]b|")
}

// Outdent is one container less for every container: a quote's block
// leaves it, splitting it where the block stood, one level at a time, as
// an item leaves its list.
func TestOutdentLeavesAQuote(t *testing.T) {
	outdent := vocab.Command{ID: "lists.outdent"}
	p := open(t)
	p.typeText("> a")
	p.do(vocab.Break{})
	p.typeText("b")
	p.do(vocab.Break{})
	p.typeText("c")
	b := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(1).ID()
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: b}, 1))})
	p.do(outdent)
	p.want("page(quote(paragraph(a)) paragraph(b) quote(paragraph(c)))")

	p = open(t)
	p.typeText("a")
	p.do(profile.WrapQuote{})
	p.do(profile.WrapQuote{})
	p.want("page(quote(quote(paragraph(a))))")
	p.do(outdent)
	p.want("page(quote(paragraph(a)))")
	p.do(outdent)
	p.want("page(paragraph(a))")

	p = open(t)
	p.typeText("> - a")
	p.do(outdent)
	p.want("page(quote(paragraph(a)))")
}

func guideFor(p *page, cmd vocab.CommandID) (present.Guide, bool) {
	for _, g := range p.e.Decorations(1).Guides {
		if g.Command == cmd {
			return g, true
		}
	}
	return present.Guide{}, false
}

func TestListNestAndOutdentGuides(t *testing.T) {
	p := twoItems(t)
	// Second item can both nest (+1) and outdent (-1).
	if g, ok := guideFor(p, "lists.nest"); !ok || g.Shift != 1 {
		t.Fatalf("second item nest guide = %+v, ok=%v", g, ok)
	}
	if g, ok := guideFor(p, "lists.outdent"); !ok || g.Shift != -1 {
		t.Fatalf("second item outdent guide = %+v, ok=%v", g, ok)
	}

	// First top-level item can only outdent (-1), not nest.
	first := p.e.Snapshot().Root().Slot(0).Child(0).Slot(0).Child(0).ID()
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: first}, 1))})
	if g, ok := guideFor(p, "lists.nest"); ok {
		t.Fatalf("first item offered nest guide: %+v", g)
	}
	if g, ok := guideFor(p, "lists.outdent"); !ok || g.Shift != -1 {
		t.Fatalf("first item outdent guide = %+v, ok=%v", g, ok)
	}

	// Inside a mark in the second item, core.leave claims Tab so lists.nest steps back;
	// after closing the mark, core.return claims Shift+Tab so lists.outdent steps back.
	p = open(t)
	p.typeText("- one")
	p.do(vocab.Break{})
	p.typeText("*em")
	if _, ok := guideFor(p, "core.leave"); !ok {
		t.Fatalf("inside mark, core.leave missing: %+v", p.e.Decorations(1).Guides)
	}
	if _, ok := guideFor(p, "lists.nest"); ok {
		t.Fatalf("inside mark, lists.nest should yield to core.leave: %+v", p.e.Decorations(1).Guides)
	}
	if g, ok := guideFor(p, "lists.outdent"); !ok || g.Shift != -1 {
		t.Fatalf("inside mark, lists.outdent missing: %+v, ok=%v", g, ok)
	}
	p.do(vocab.Leave{})
	if _, ok := guideFor(p, "core.return"); !ok {
		t.Fatalf("after mark, core.return missing: %+v", p.e.Decorations(1).Guides)
	}
	if _, ok := guideFor(p, "lists.outdent"); ok {
		t.Fatalf("after mark, lists.outdent should yield to core.return: %+v", p.e.Decorations(1).Guides)
	}
	if g, ok := guideFor(p, "lists.nest"); !ok || g.Shift != 1 {
		t.Fatalf("after mark, lists.nest missing: %+v, ok=%v", g, ok)
	}
}

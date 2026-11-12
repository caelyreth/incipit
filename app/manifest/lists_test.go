package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
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

package manifest

import (
	"slices"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// offered are the offers on surface, by command, and the groups offered.
func (p *page) offered(surface vocab.Surface) (map[vocab.CommandID]engine.Offered, []string) {
	out := map[vocab.CommandID]engine.Offered{}
	var groups []string
	for _, g := range p.e.Offers(1, surface) {
		groups = append(groups, g.Group.ID)
		for _, o := range g.Offers {
			out[o.Command] = o
		}
	}
	return out, groups
}

// What suits the selection is offered: a word its marks and its rungs,
// never the kinds blocks turn into, which a caret or a whole block is
// offered, its own kind on.
func TestOffersSuitWhatIsSelected(t *testing.T) {
	p := open(t)
	p.typeText("Tide pools. Shore birds.")
	ref, _ := vocab.FirstTextSlot(p.e.Snapshot().Root())

	p.selectText(ref, 5, 10)
	offers, groups := p.offered(vocab.SurfaceTools)
	if !slices.Equal(groups, []string{"marks", "selection"}) {
		t.Fatalf("A word is offered %v, want its marks and its rungs.", groups)
	}
	if o := offers["formats.strong"]; !o.Available || o.On || o.Key != "b" {
		t.Fatalf("Strong over a plain word is %+v.", o)
	}
	p.do(profile.Format{Mark: profile.Strong})
	if offers, _ = p.offered(vocab.SurfaceTools); !offers["formats.strong"].On {
		t.Fatal("Strong over a strong word is not on.")
	}

	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(ref, 2))})
	offers, groups = p.offered(vocab.SurfaceTools)
	if !slices.Contains(groups, "kind") || !offers["blocks.paragraph"].On || offers["blocks.heading1"].On {
		t.Fatalf("A caret in a paragraph is offered %v, paragraph %+v.", groups, offers["blocks.paragraph"])
	}
	p.selectText(ref, 0, 24)
	if _, groups = p.offered(vocab.SurfaceTools); !slices.Contains(groups, "kind") {
		t.Fatalf("A whole block is offered %v, without its kind.", groups)
	}
	if _, groups = p.offered(vocab.SurfaceDock); !slices.Equal(groups, []string{"insert", "kind", "marks"}) {
		t.Fatalf("The dock is offered %v.", groups)
	}
}

// The items of a list selected are offered their list's kind, on; a
// heading in a quote is both.
func TestOffersNameTheKindsSelected(t *testing.T) {
	p := open(t)
	p.typeText("- a")
	p.do(vocab.Break{})
	p.typeText("b")
	items := p.e.Snapshot().Root().Slot(0).Children()[0].Slot(0).Children()
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: items[0].ID()}, 0))})
	p.do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: items[1].ID()}, 1)), Extend: true})
	offers, _ := p.offered(vocab.SurfaceTools)
	if !offers["lists.bullet"].On || offers["lists.numbered"].On || offers["blocks.paragraph"].On {
		t.Fatalf("Items of a bulleted list: bullet %v, numbered %v, paragraph %v.",
			offers["lists.bullet"].On, offers["lists.numbered"].On, offers["blocks.paragraph"].On)
	}
}

// A table's menu offers what acts on whole rows where they are selected,
// on whole columns where they are, and both in a cell.
func TestTableOffersFollowTheLinesSelected(t *testing.T) {
	p := open(t)
	table := p.placeTable()
	cell := cellAt(table, 0, 0)
	p.do(vocab.Place{Stop: doc.CaretAt(doc.Pos{Node: cell.ID()})})
	offers, _ := p.offered(vocab.SurfaceMenu)
	for _, id := range []vocab.CommandID{"table.row-above", "table.column-left", "table.delete"} {
		if !offers[id].Available {
			t.Fatalf("A cell is not offered %s.", id)
		}
	}
	p.do(vocab.Place{Cells: &doc.CellRange{Table: table.ID(), WholeRows: true}})
	offers, _ = p.offered(vocab.SurfaceMenu)
	if _, ok := offers["table.column-left"]; ok || !offers["table.delete-row"].Available {
		t.Fatalf("Whole rows are offered %v.", offers)
	}
	if _, ok := offers["table.delete"]; ok {
		t.Fatal("Whole rows are offered the table's deletion.")
	}
}

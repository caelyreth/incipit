package manifest

import (
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// placeTable puts a two-by-two table of cells a, b, c, d on the page.
func (p *page) placeTable() *doc.Node {
	p.t.Helper()
	cell := func(t string) *doc.Node {
		return doc.NewNode(profile.Cell, doc.Attrs{}, doc.InlineSlot(doc.Item{Text: t}))
	}
	row := func(a, b string) *doc.Node {
		return doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(cell(a), cell(b)))
	}
	cols := doc.MakeAttrs(doc.Field{Num: profile.FieldColumns, Value: doc.List(doc.Attrs{}, doc.Attrs{})})
	p.do(vocab.Paste{Slice: vocab.Slice{Nodes: []*doc.Node{doc.NewNode(profile.Table, cols, doc.BlockSlot(row("a", "b"), row("c", "d")))}}})
	return p.e.Snapshot().Root().Slot(0).Child(0)
}

func cellAt(table *doc.Node, row, col int) *doc.Node {
	return table.Slot(0).Child(row).Slot(0).Child(col)
}

// A table's cells are its text: a caret stands in them, and deleting at a
// cell's edge neither merges cells nor lifts one out of its row.
func TestATablesCellsAreTextThatKeepsItsGrid(t *testing.T) {
	for _, tc := range []struct {
		row, col, at int
		dir          vocab.Direction
	}{
		{0, 1, 0, vocab.Backward}, {0, 0, 0, vocab.Backward}, {0, 0, 1, vocab.Forward}, {1, 0, 0, vocab.Backward},
	} {
		p := open(t)
		table := p.placeTable()
		cell := cellAt(table, tc.row, tc.col)
		p.do(vocab.Place{Stop: doc.CaretAt(doc.Pos{Node: cell.ID(), Offset: tc.at})})
		if h := p.selection().Head.Pos; h.Node != cell.ID() {
			t.Fatalf("caret at %v, want in cell %d,%d", h, tc.row, tc.col)
		}
		p.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: tc.dir})
		p.want("page(table(row(cell(a) cell(b)) row(cell(c) cell(d))))")
	}
}

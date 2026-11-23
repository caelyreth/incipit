package clipboard

import (
	"testing"

	"github.com/caelyreth/neverland/internal/app/markdown"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// Text another application copied as a table is one: a spreadsheet's
// lines between tabs, or a Markdown table with its columns' alignment.
func TestTableTextReadsSpreadsheetsAndMarkdown(t *testing.T) {
	s, ok := TableText("a\tb\nc\td\n")
	if !ok || len(s.Nodes) != 1 || profile.TableShapeOf(s.Nodes[0]).Header {
		t.Fatalf("tabs: %v %+v", ok, s)
	}
	if got := len(s.Nodes[0].Slot(0).Children()); got != 2 {
		t.Fatalf("tabs make %d rows", got)
	}
	s, ok = TableText("| Tide | Height |\n| :-- | --: |\n| high | 4.2 |")
	if !ok {
		t.Fatal("markdown table not read")
	}
	shape := profile.TableShapeOf(s.Nodes[0])
	if !shape.Header || shape.Columns[0].Align != profile.AlignStart || shape.Columns[1].Align != profile.AlignEnd {
		t.Fatalf("shape %+v", shape)
	}
	for _, text := range []string{"one line", "| a |\nno delimiter", ""} {
		if _, ok := TableText(text); ok {
			t.Errorf("%q read as a table", text)
		}
	}
}

func marksOf(kinds ...doc.MarkKind) *doc.MarkSet {
	in := doc.NewInterner()
	ms := in.Empty()
	for _, k := range kinds {
		ms = ms.With(doc.Mark{Kind: k})
	}
	return ms
}

func TestTableTextReadsMarkdownTablesBack(t *testing.T) {
	s := profile.Schema()
	table := profile.TableOf([][]string{{"Left", "Mid", "Right"}, {"a | b", "c", "d"}})
	shape := profile.TableShapeOf(table)
	shape.Columns[0].Align = profile.AlignStart
	shape.Columns[1].Align = profile.AlignCenter
	shape.Columns[2].Align = profile.AlignEnd
	rows := table.Slot(0).Children()
	rows[1] = doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("a | b", nil)),
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("c", marksOf(profile.Strong))),
		vocab.NewBlock(s, profile.Cell, doc.Attrs{}, doc.TextItem("d", nil)),
	))
	aligned := doc.NewNode(profile.Table, shape.Attrs(), doc.BlockSlot(rows...))

	md := markdown.Slice(vocab.Slice{Nodes: []*doc.Node{aligned}})
	want := "| Left | Mid | Right |\n| --- | :---: | ---: |\n| a \\| b | **c** | d |"
	if md != want {
		t.Fatalf("table = %q, want %q", md, want)
	}
	back, ok := TableText(md)
	if !ok || len(back.Nodes) != 1 {
		t.Fatalf("TableText failed to parse back %q", md)
	}
	gotShape := profile.TableShapeOf(back.Nodes[0])
	if len(gotShape.Columns) != 3 ||
		gotShape.Columns[0].Align != profile.AlignStart ||
		gotShape.Columns[1].Align != profile.AlignCenter ||
		gotShape.Columns[2].Align != profile.AlignEnd {
		t.Fatalf("round-trip alignments = %+v", gotShape.Columns)
	}
}

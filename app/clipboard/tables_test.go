package clipboard

import (
	"testing"

	"github.com/caelyreth/neverland/editor/profile"
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

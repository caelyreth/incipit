package textlayout

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// testTable is a flow of one table row, a cell for each text.
func testTable(t *testing.T, texts []string) *surface.Flow {
	t.Helper()
	text := strings.Join(texts, "\n")
	table := surface.Table{Rows: [][]surface.TableCell{{}}}
	at := 0
	for index, cell := range texts {
		units := model.StringLenUTF16(cell)
		table.Rows[0] = append(table.Rows[0], surface.TableCell{
			Node:  model.NodeID(index + 1),
			Range: model.NewUTF16Range(model.UTF16Offset(at), model.UTF16Offset(at+units)),
		})
		table.Align = append(table.Align, surface.CellStart)
		table.Fit = append(table.Fit, false)
		at += units + 1
	}
	flow, err := testFlow(t, text, 0).WithTables([]surface.Table{table})
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

// A table whose text fits keeps to the column; one whose columns ask for
// more than it even wrapped reaches into the margins as far on each side,
// and past them is scrolled across, its cells cut off at the room it shows
// in.
func TestAWideTableSpreadsBothWaysThenScrolls(t *testing.T) {
	style := testStyle()
	style.Spread = 40
	narrow := NewCache().Update(testTable(t, []string{"a", "b"}), nil, 400, style).Tables()[0]
	if narrow.Box.X > 0 || narrow.Box.W > 401 || narrow.Overflow != 0 {
		t.Fatalf("a narrow table stands at %+v, overflowing %v", narrow.Box, narrow.Overflow)
	}
	words := []string{}
	for range 6 {
		words = append(words, "a heading long enough to want room")
	}
	wide := NewCache().Update(testTable(t, words), nil, 400, style).Tables()[0]
	if wide.Box.X != narrow.Box.X-40 || wide.Box.W != narrow.Box.W+80 {
		t.Fatalf("a wide table shows in %+v, not spread 40 each way", wide.Box)
	}
	if wide.Overflow <= 0 || wide.Grid.W != wide.Box.W+wide.Overflow || wide.Grid.X != wide.Box.X {
		t.Fatalf("a wide table's grid %+v overflows %v", wide.Grid, wide.Overflow)
	}
	scrolled := NewCache().Update(testTable(t, words), &Overlay{Tables: map[model.NodeID]TableRoom{1: {Scroll: 1e6}}}, 400, style)
	placed := scrolled.Tables()[0]
	if placed.Scroll != placed.Overflow || placed.Grid.X != placed.Box.X-placed.Overflow {
		t.Fatalf("scrolled past its end, the table stands at %+v scrolled %v", placed.Grid, placed.Scroll)
	}
	for _, paragraph := range scrolled.Paragraphs() {
		if clip, ok := paragraph.Clip(); !ok || clip != placed.Box {
			t.Fatalf("a cell of a wide table shows in %+v", clip)
		}
	}
}

// A table whose text fits has even columns; one whose text asks for more
// than the column wraps within it, its short column keeping its line, and
// room to write more, and its long one taking the rest.
func TestALongCellWrapsWithinTheColumn(t *testing.T) {
	style := testStyle()
	style.Spread = 40
	long := strings.Repeat("words that run on ", 8)
	short := NewCache().Update(testTable(t, []string{"short", ""}), nil, 400, style).Tables()[0]
	if short.Widths[0] != 200 || short.Widths[1] != 200 {
		t.Fatalf("a table whose text fits has columns %v", short.Widths)
	}
	placed := NewCache().Update(testTable(t, []string{"short", long}), nil, 400, style).Tables()[0]
	if placed.Box.W > 401 || placed.Overflow != 0 {
		t.Fatalf("the table stands at %+v, overflowing %v", placed.Box, placed.Overflow)
	}
	if placed.Widths[0] < placed.Natural[0] || placed.Widths[0] >= placed.Widths[1] {
		t.Fatalf("the short column is %v wide, asking %v, by %v", placed.Widths[0], placed.Natural[0], placed.Widths[1])
	}
	if placed.Widths[0]+placed.Widths[1] != 400 {
		t.Fatalf("the columns are %v wide in a column of 400", placed.Widths)
	}
}

// Empty columns keep room to write in, however many there are: the table
// reaches into the margins rather than thin them.
func TestEmptyColumnsKeepRoomToWrite(t *testing.T) {
	style := testStyle()
	style.Spread = 40
	placed := NewCache().Update(testTable(t, make([]string, 8)), nil, 400, style).Tables()[0]
	read := style.Prose.LineHeight() * readColumn
	for _, width := range placed.Widths {
		if width < read-1 {
			t.Fatalf("empty columns are %v wide, under %v", placed.Widths, read)
		}
	}
	if placed.Box.W != 480 || placed.Overflow <= 0 {
		t.Fatalf("the table shows in %+v, overflowing %v", placed.Box, placed.Overflow)
	}
}

// A column set to fit its text keeps it on one line, the others wrapping
// in what it leaves.
func TestAFittedColumnKeepsItsLine(t *testing.T) {
	style := testStyle()
	long := strings.Repeat("words that run on ", 4)
	flow := testTable(t, []string{long, long})
	tables := flow.Tables()
	tables[0].Fit[0] = true
	flow, err := flow.WithTables(tables)
	if err != nil {
		t.Fatal(err)
	}
	placed := NewCache().Update(flow, nil, 400, style).Tables()[0]
	if placed.Widths[0] != placed.Natural[0] || placed.Widths[1] >= placed.Natural[1] {
		t.Fatalf("the columns are %v, asking %v", placed.Widths, placed.Natural)
	}
}

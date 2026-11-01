package textlayout

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// testTable is a flow of one table row, a cell for each text.
func testTable(t *testing.T, texts []string, room surface.TableRoom) *surface.Flow {
	t.Helper()
	text := strings.Join(texts, "\n")
	table := surface.Table{Rows: [][]surface.TableCell{{}}, Room: room}
	at := 0
	for index, cell := range texts {
		units := model.StringLenUTF16(cell)
		table.Rows[0] = append(table.Rows[0], surface.TableCell{
			Node:  model.NodeID(index + 1),
			Range: model.NewUTF16Range(model.UTF16Offset(at), model.UTF16Offset(at+units)),
		})
		table.Align = append(table.Align, surface.CellStart)
		table.Widths = append(table.Widths, 0)
		at += units + 1
	}
	flow, err := testFlow(t, text, 0).WithTables([]surface.Table{table})
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

// A table no wider than its text keeps to the column; one wider reaches
// into the margins as far on each side, and past them is scrolled across,
// its cells cut off at the room it shows in.
func TestAWideTableSpreadsBothWaysThenScrolls(t *testing.T) {
	style := testStyle()
	style.Spread = 40
	narrow := NewCache().Update(testTable(t, []string{"a", "b"}, surface.TableRoom{}), 400, style).Tables()[0]
	if narrow.Box.X > 0 || narrow.Box.W > 401 || narrow.Overflow != 0 {
		t.Fatalf("a narrow table stands at %+v, overflowing %v", narrow.Box, narrow.Overflow)
	}
	words := []string{}
	for range 6 {
		words = append(words, "a heading long enough to want room")
	}
	wide := NewCache().Update(testTable(t, words, surface.TableRoom{}), 400, style).Tables()[0]
	if wide.Box.X != narrow.Box.X-40 || wide.Box.W != narrow.Box.W+80 {
		t.Fatalf("a wide table shows in %+v, not spread 40 each way", wide.Box)
	}
	if wide.Overflow <= 0 || wide.Grid.W != wide.Box.W+wide.Overflow || wide.Grid.X != wide.Box.X {
		t.Fatalf("a wide table's grid %+v overflows %v", wide.Grid, wide.Overflow)
	}
	scrolled := NewCache().Update(testTable(t, words, surface.TableRoom{Scroll: 1e6}), 400, style)
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

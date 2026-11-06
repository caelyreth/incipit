package surface

import (
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func textFlow(t *testing.T, text string) *Flow {
	t.Helper()
	flow, err := NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(len(text))), Style: TextStyle{}}},
		lineBlocks(text),
	)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

// addTable appends a two-row table to prefix. Cell ranges cover their text,
// not the generated line break after it, as the editor's projected cells do.
func addTable(t *testing.T, prefix string, rows [][]string) (*Flow, Table) {
	t.Helper()
	start := model.UTF16Offset(model.StringLenUTF16(prefix))
	table := Table{
		Header: true,
		Align:  []CellAlign{CellStart, CellCenter},
		Fit:    []bool{false, true},
	}
	at := start
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]TableCell, 0, len(row))
		for _, content := range row {
			lines = append(lines, content)
			units := model.UTF16Offset(model.StringLenUTF16(content))
			cells = append(cells, TableCell{
				Node:  model.NodeID(len(table.Rows)*len(row) + len(cells) + 1),
				Range: model.NewUTF16Range(at, at+units),
			})
			at += units + 1
		}
		table.Rows = append(table.Rows, cells)
	}
	flow, err := textFlow(t, prefix+strings.Join(lines, "\n")+"\n").WithTables([]Table{table})
	if err != nil {
		t.Fatal(err)
	}
	return flow, table
}

func previewFrame(t *testing.T, flow *Flow, original model.UTF16Range, text string) *Frame {
	t.Helper()
	frame, err := NewFrame(flow, model.EmptyUTF16Range(original.Start), original.Start, TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := frame.WithPreview(original, text, model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(text))))
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func movedTable(table Table, delta model.UTF16Offset) Table {
	moved := table
	moved.Rows = make([][]TableCell, len(table.Rows))
	for row := range table.Rows {
		moved.Rows[row] = make([]TableCell, len(table.Rows[row]))
		for column, cell := range table.Rows[row] {
			moved.Rows[row][column] = TableCell{
				Node: cell.Node,
				Range: model.NewUTF16Range(
					cell.Range.Start+delta,
					cell.Range.End+delta,
				),
			}
		}
	}
	return moved
}

func TestPreviewRemapsTablesBeforeInsideAndAfter(t *testing.T) {
	t.Parallel()
	flow, table := addTable(t, "intro\n", [][]string{{"A🦀", "B"}, {"C", "D"}})
	after := model.UTF16Offset(model.StringLenUTF16(flow.Text()))

	t.Run("before table with astral preedit", func(t *testing.T) {
		preview := previewFrame(t, flow, model.EmptyUTF16Range(2), "🦀")
		tables := preview.Flow().Tables()
		if len(tables) != 1 {
			t.Fatalf("preview tables = %d", len(tables))
		}
		want := movedTable(table, 2)
		if !reflect.DeepEqual(tables[0], want) {
			t.Fatalf("preview table = %#v, want %#v", tables[0], want)
		}
	})

	t.Run("inside one cell", func(t *testing.T) {
		original := table.Rows[1][0].Range
		preview := previewFrame(t, flow, original, "界🦀")
		tables := preview.Flow().Tables()
		if len(tables) != 1 {
			t.Fatalf("preview tables = %d", len(tables))
		}
		want := movedTable(table, 0)
		want.Rows[1][0].Range.End += 2
		want.Rows[1][1].Range = model.NewUTF16Range(
			table.Rows[1][1].Range.Start+2,
			table.Rows[1][1].Range.End+2,
		)
		if !reflect.DeepEqual(tables[0], want) {
			t.Fatalf("preview table = %#v, want %#v", tables[0], want)
		}
	})

	t.Run("after table with astral preedit", func(t *testing.T) {
		preview := previewFrame(t, flow, model.EmptyUTF16Range(after), "🦀")
		tables := preview.Flow().Tables()
		if len(tables) != 1 || !reflect.DeepEqual(tables[0], table) {
			t.Fatalf("preview table = %#v, want %#v", tables, table)
		}
	})
}

func TestPreviewInsertionAtCellEndBelongsToCell(t *testing.T) {
	flow, table := addTable(t, "", [][]string{{"A", ""}, {"C", "D"}})
	for _, target := range []struct {
		name        string
		row, column int
	}{
		{"nonempty cell", 0, 0},
		{"empty cell", 0, 1},
		{"last cell", 1, 1},
	} {
		t.Run(target.name, func(t *testing.T) {
			cell := table.Rows[target.row][target.column]
			preview := previewFrame(t, flow, model.EmptyUTF16Range(cell.Range.End), "x")
			tables := preview.Flow().Tables()
			if len(tables) != 1 {
				t.Fatalf("preview tables = %d", len(tables))
			}
			want := movedTable(table, 0)
			for row := range want.Rows {
				for column := range want.Rows[row] {
					got := &want.Rows[row][column]
					switch {
					case row == target.row && column == target.column:
						got.Range.End++
					case got.Range.Start > cell.Range.End:
						got.Range.Start++
						got.Range.End++
					}
				}
			}
			if !reflect.DeepEqual(tables[0], want) {
				t.Fatalf("preview table = %#v, want %#v", tables[0], want)
			}
		})
	}
}

func TestPreviewDropsTableMetadataForCrossCellComposition(t *testing.T) {
	t.Parallel()
	flow, _ := addTable(t, "", [][]string{{"A", "B"}, {"C", "D"}})
	// A semantic composition cannot cross cells. A direct surface preview may
	// synthesize that shape, but must not stretch either cell's node range
	// through the other cell.
	preview := previewFrame(t, flow, model.NewUTF16Range(0, 4), "x\ny\n")
	if got := preview.Flow().Text(); got != "x\ny\nC\nD\n" {
		t.Fatalf("preview text = %q", got)
	}
	if tables := preview.Flow().Tables(); len(tables) != 0 {
		t.Fatalf("cross-cell preview tables = %#v", tables)
	}
}

func TestPreviewRemapsFiguresAroundCaptionEdits(t *testing.T) {
	t.Parallel()
	text := "intro\n\uFFFC\ncaption\nout"
	flow := textFlow(t, text)
	at := model.UTF16Offset(model.StringLenUTF16("intro\n"))
	figure := Figure{
		Range: model.NewUTF16Range(at, at+1), Node: 2, Media: "media/1", Width: 100, Height: 50,
		Span: 0.5, Align: FigureStart, Alt: "A crab", Captioned: true,
	}
	flow, err := ownedBy(flow, asObjects([]Figure{figure}))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("before figure with astral preedit", func(t *testing.T) {
		preview := previewFrame(t, flow, model.EmptyUTF16Range(2), "🦀")
		figures := ObjectsOf[Figure](preview.Flow())
		want := figure
		want.Range.Start, want.Range.End = want.Range.Start+2, want.Range.End+2
		if !reflect.DeepEqual(figures, []Figure{want}) {
			t.Fatalf("preview figures = %#v, want %#v", figures, want)
		}
	})

	t.Run("inside caption", func(t *testing.T) {
		caption := figure.CaptionStart()
		preview := previewFrame(t, flow, model.NewUTF16Range(caption, caption+3), "界")
		if figures := ObjectsOf[Figure](preview.Flow()); !reflect.DeepEqual(figures, []Figure{figure}) {
			t.Fatalf("preview figures = %#v, want %#v", figures, figure)
		}
	})

	t.Run("after figure", func(t *testing.T) {
		after := model.UTF16Offset(model.StringLenUTF16(flow.Text()))
		preview := previewFrame(t, flow, model.EmptyUTF16Range(after), "🦀")
		if figures := ObjectsOf[Figure](preview.Flow()); !reflect.DeepEqual(figures, []Figure{figure}) {
			t.Fatalf("preview figures = %#v, want %#v", figures, figure)
		}
	})

	t.Run("replacing the object drops it", func(t *testing.T) {
		preview := previewFrame(t, flow, figure.Range, "x")
		if figures := ObjectsOf[Figure](preview.Flow()); len(figures) != 0 {
			t.Fatalf("preview figures = %#v", figures)
		}
	})
}

func TestPreviewKeepsCompositionDecorationPolicy(t *testing.T) {
	t.Parallel()
	flow := fixtureFlow(t)
	frame, err := NewFrame(
		flow,
		model.EmptyUTF16Range(1),
		1,
		TextStyle{Italic: true},
		[]Decoration{{
			ID:         DecorationID{Producer: "editor.pairs", Local: 1},
			Primitive:  FillPrimitive([]model.UTF16Range{model.NewUTF16Range(0, 3)}, ColorMuted, 4),
			Opacity:    FixedOpacity(24),
			Visibility: VisibilityFocused,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := frame.WithPreview(model.EmptyUTF16Range(1), "界", model.EmptyUTF16Range(1))
	if err != nil {
		t.Fatal(err)
	}
	decorations := preview.Decorations()
	if len(decorations) != 1 || decorations[0].ID != (DecorationID{Producer: "surface.preview"}) {
		t.Fatalf("preview decorations = %#v", decorations)
	}
	if got := decorations[0].Primitive.Ranges; !reflect.DeepEqual(got, []model.UTF16Range{model.NewUTF16Range(1, 2)}) {
		t.Fatalf("marked decoration ranges = %#v", got)
	}
}

func TestPreviewRejectsSplitSurrogateBoundaries(t *testing.T) {
	t.Parallel()
	flow := textFlow(t, "a🦀b")
	frame, err := NewFrame(flow, model.EmptyUTF16Range(1), 1, TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frame.WithPreview(model.EmptyUTF16Range(2), "x", model.EmptyUTF16Range(0)); err == nil {
		t.Fatal("split-surrogate edit start accepted")
	}
	if _, err := frame.WithPreview(model.EmptyUTF16Range(1), "🦀", model.NewUTF16Range(1, 2)); err == nil {
		t.Fatal("split-surrogate preedit selection accepted")
	}
}

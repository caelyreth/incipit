package surface

import (
	"fmt"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func fixtureFlow(t *testing.T) *Flow {
	t.Helper()
	flow, err := NewFlow(
		projection.NewVersion(1, 2),
		"a🦀bc",
		[]StyleRun{
			{Bytes: model.NewByteRange(0, 5), Style: TextStyle{}},
			{Bytes: model.NewByteRange(5, 7), Style: TextStyle{Bold: true}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func TestFlowRejectsGapsOverlapsAndSplitUnicode(t *testing.T) {
	t.Parallel()
	for _, bytes := range []model.ByteRange{
		model.NewByteRange(0, 1),
		model.NewByteRange(1, 7),
		model.NewByteRange(0, 2),
		model.NewByteRange(7, 0),
	} {
		if _, err := NewFlow(
			projection.NewVersion(0, 0),
			"a🦀bc",
			[]StyleRun{{Bytes: bytes}},
		); err == nil {
			t.Fatalf("invalid run accepted: %#v", bytes)
		}
	}
	if _, err := NewFrame(
		fixtureFlow(t),
		model.NewUTF16Range(2, 2),
		model.UTF16Offset(2),
		TextStyle{},
		nil,
	); err == nil {
		t.Fatal("split-surrogate selection accepted")
	}
}

func TestFlowEqualIncludesPresentationMetadata(t *testing.T) {
	flow, table := addTable(t, "", [][]string{{"A", "B"}, {"C", "D"}})
	copyOf := func() *Flow {
		copy, err := flow.WithTables(flow.Tables())
		if err != nil {
			t.Fatal(err)
		}
		return copy
	}
	if !flow.Equal(copyOf()) || flow.Equal(nil) || (*Flow)(nil).Equal(flow) {
		t.Fatal("equal flows or nil handling differ")
	}
	for _, change := range []struct {
		name string
		edit func(*Flow)
	}{
		{"version", func(f *Flow) { f.version = projection.NewVersion(9, 2) }},
		{"text", func(f *Flow) { f.text += "x" }},
		{"style", func(f *Flow) { f.runs = []StyleRun{{Style: TextStyle{Bold: true}}} }},
		{"figure", func(f *Flow) { f.figures = []Figure{{Node: 99}} }},
		{"room", func(f *Flow) { f.rooms = []Room{{At: -1, Height: 2}} }},
		{"table node", func(f *Flow) {
			f.tables[0].Rows = [][]TableCell{{{Node: 99, Range: table.Rows[0][0].Range}}}
		}},
		{"table fit", func(f *Flow) { f.tables[0].Fit = []bool{true, false} }},
		{"table room", func(f *Flow) { f.tables[0].Room.Scroll++ }},
		{"table row reveal", func(f *Flow) { f.tables[0].Room.RowShown = []float32{1, 0.5} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			other := copyOf()
			change.edit(other)
			if flow.Equal(other) {
				t.Fatal("different flow compared equal")
			}
		})
	}
}

func TestFlowOwnsNestedTableValues(t *testing.T) {
	base := textFlow(t, "a\nb")
	table := Table{
		Rows:  [][]TableCell{{{Node: 1, Range: model.NewUTF16Range(0, 1)}, {Node: 2, Range: model.NewUTF16Range(2, 3)}}},
		Align: []CellAlign{CellStart, CellEnd},
		Fit:   []bool{true, false},
		Room:  TableRoom{RowShown: []float32{1}, ColumnsApart: []float32{0, 4, 0}},
	}
	flow, err := base.WithTables([]Table{table})
	if err != nil {
		t.Fatal(err)
	}
	table.Rows[0][0].Node = 99
	table.Fit[0] = false
	table.Room.RowShown[0] = 0
	tables := flow.Tables()
	tables[0].Align[0] = CellCenter
	tables[0].Room.ColumnsApart[1] = 99
	at, ok := flow.TableAt(0)
	if !ok {
		t.Fatal("table missing")
	}
	at.Rows[0][1].Node = 99
	got := flow.Tables()[0]
	if got.Rows[0][0].Node != 1 || got.Rows[0][1].Node != 2 || !got.Fit[0] ||
		got.Align[0] != CellStart || got.Room.RowShown[0] != 1 || got.Room.ColumnsApart[1] != 4 {
		t.Fatalf("flow table changed through an external slice: %+v", got)
	}
}

func TestEachRunVisitsRunsInOrderAndStopsEarly(t *testing.T) {
	t.Parallel()
	flow, err := NewFlow(
		projection.NewVersion(1, 2),
		"abc",
		[]StyleRun{
			{
				Bytes: model.NewByteRange(0, 1),
				Style: TextStyle{Bold: true},
			},
			{
				Bytes: model.NewByteRange(1, 3),
				Style: TextStyle{Italic: true},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var ranges []string
	flow.EachRun(func(run StyleRun) bool {
		ranges = append(ranges, fmt.Sprintf("%d:%d", run.Bytes.Start, run.Bytes.End))
		return true
	})
	if !slices.Equal(ranges, []string{"0:1", "1:3"}) {
		t.Fatalf("runs = %v", ranges)
	}
	ranges = nil
	flow.EachRun(func(run StyleRun) bool {
		ranges = append(ranges, fmt.Sprintf("%d:%d", run.Bytes.Start, run.Bytes.End))
		return false
	})
	if !slices.Equal(ranges, []string{"0:1"}) {
		t.Fatalf("early runs = %v", ranges)
	}
}

func TestPreviewPreservesSurroundingRuns(t *testing.T) {
	t.Parallel()
	flow := fixtureFlow(t)
	original, err := NewFrame(
		flow,
		model.NewUTF16Range(1, 4),
		model.UTF16Offset(4),
		TextStyle{Italic: true},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := original.WithPreview(
		model.NewUTF16Range(1, 4),
		"你",
		model.NewUTF16Range(1, 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Flow().Text() != "a你c" {
		t.Fatalf("preview = %q", preview.Flow().Text())
	}
	if preview.Selection() != model.NewUTF16Range(2, 2) {
		t.Fatalf("selection = %#v", preview.Selection())
	}
	runs := preview.Flow().Runs()
	if len(runs) != 3 || runs[0].Bytes != model.NewByteRange(0, 1) ||
		runs[1].Bytes != model.NewByteRange(1, 4) ||
		runs[2].Bytes != model.NewByteRange(4, 5) {
		t.Fatalf("runs = %#v", runs)
	}
	if !runs[1].Style.Italic || !runs[2].Style.Bold {
		t.Fatalf("preview styles = %#v", runs)
	}
	if flow.Text() != "a🦀bc" {
		t.Fatal("preview mutated source")
	}
}

func TestPreviewRemapsFigureMetadata(t *testing.T) {
	t.Parallel()
	text := "intro\n\uFFFC\ncaption"
	flow, err := NewFlow(
		projection.NewVersion(1, 1),
		text,
		[]StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: TextStyle{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	at := model.UTF16Offset(model.StringLenUTF16("intro\n"))
	flow, err = flow.WithFigures([]Figure{{
		Object: model.NewUTF16Range(at, at+1), Node: 2, Media: "media/1", Width: 100, Height: 50,
	}})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := NewFrame(flow, model.EmptyUTF16Range(at), at, TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := frame.WithPreview(model.EmptyUTF16Range(0), "x", model.EmptyUTF16Range(0))
	if err != nil {
		t.Fatal(err)
	}
	figures := preview.Flow().Figures()
	if len(figures) != 1 || figures[0].Object.Start != at+1 || figures[0].Media != "media/1" {
		t.Fatalf("preview figures = %#v", figures)
	}
}

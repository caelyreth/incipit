package surface

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// A joined flow is its parts one after another, each block, run and
// object moved with its part, a break between each in its style; runs
// alike across a break are one.
func TestJoinMovesEachPartWhole(t *testing.T) {
	plain := TextStyle{Color: ColorText}
	part := func(text string, objects ...Object) *Flow {
		t.Helper()
		var runs []StyleRun
		if text != "" {
			runs = []StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(len(text))), Style: plain}}
		}
		flow, err := NewFlow(projection.NewVersion(0, 0), text, runs, []Block{{Node: objectNode(objects), Range: model.NewUTF16Range(0, model.UTF16Offset(model.StringLenUTF16(text)))}})
		if err == nil {
			flow, err = flow.WithObjects(objects)
		}
		if err != nil {
			t.Fatal(err)
		}
		return flow
	}
	// A formula whose source starts its part moves whole, not stretched.
	first := part("x$", Formula{Node: 1, Range: model.NewUTF16Range(0, 2), Text: model.NewUTF16Range(0, 1), Source: "x"})
	second := part("")
	third := part("\uFFFC🦀", Figure{Node: 3, Range: model.NewUTF16Range(0, 1), Width: 1, Height: 1})
	flow, err := Join(projection.NewVersion(4, 2), []*Flow{first, second, third}, []TextStyle{plain, {Color: ColorMuted}})
	if err != nil {
		t.Fatal(err)
	}
	if flow.Text() != "x$\n\n\uFFFC🦀" || flow.Version() != projection.NewVersion(4, 2) {
		t.Fatalf("joined %q at %v", flow.Text(), flow.Version())
	}
	blocks := flow.Blocks()
	if len(blocks) != 3 || blocks[1].Range != model.EmptyUTF16Range(3) || blocks[2].Range != model.NewUTF16Range(4, 7) {
		t.Fatalf("blocks = %+v", blocks)
	}
	runs := flow.Runs()
	if len(runs) != 3 || runs[0].Bytes != model.NewByteRange(0, 3) || runs[1].Style.Color != ColorMuted {
		t.Fatalf("runs = %+v", runs)
	}
	f, ok := ObjectOf[Formula](flow, 0)
	figure, found := ObjectOf[Figure](flow, 4)
	if !ok || f.Range != model.NewUTF16Range(0, 2) || f.Text != model.NewUTF16Range(0, 1) ||
		!found || figure.Range != model.NewUTF16Range(4, 5) {
		t.Fatalf("objects = %+v", flow.Objects())
	}
	if _, err := Join(projection.NewVersion(0, 0), []*Flow{first, second}, nil); err == nil {
		t.Fatal("a join without its break was taken")
	}
	tabled, err := textFlow(t, "a").WithTables([]Table{{Rows: [][]TableCell{{{Node: 1, Range: model.NewUTF16Range(0, 1)}}}, Align: []CellAlign{CellStart}, Fit: []bool{false}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Join(projection.NewVersion(0, 0), []*Flow{tabled}, nil); err == nil {
		t.Fatal("a part with a table was joined")
	}
}

// objectNode is the node objects are the notation of, or the first
// block's.
func objectNode(objects []Object) model.NodeID {
	if len(objects) > 0 {
		return objects[0].node()
	}
	return 2
}

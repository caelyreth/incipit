package surface

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// A flow's blocks cover its text in order, one line break between each and
// the next; a block may hold several lines.
func TestBlocksCoverTheText(t *testing.T) {
	text := "a\nb\nc"
	runs := []StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(len(text)))}}
	for _, good := range [][]Block{
		lineBlocks(text),
		{{Range: model.NewUTF16Range(0, 3)}, {Range: model.NewUTF16Range(4, 5)}},
		{{Range: model.NewUTF16Range(0, 5)}},
	} {
		if _, err := NewFlow(projection.NewVersion(1, 1), text, runs, good); err != nil {
			t.Fatalf("blocks %+v refused: %v", good, err)
		}
	}
	for _, bad := range [][]Block{
		nil,
		{{Range: model.NewUTF16Range(0, 1)}, {Range: model.NewUTF16Range(2, 3)}},
		{{Range: model.NewUTF16Range(0, 2)}, {Range: model.NewUTF16Range(3, 5)}},
		{{Range: model.NewUTF16Range(1, 5)}},
	} {
		if _, err := NewFlow(projection.NewVersion(1, 1), text, runs, bad); err == nil {
			t.Fatalf("blocks %+v taken", bad)
		}
	}
	flow := textFlow(t, text)
	if block, index, ok := flow.BlockAt(3); !ok || index != 1 || block.Range != model.NewUTF16Range(2, 3) {
		t.Fatalf("the break after b is b's: %+v %d %v", block, index, ok)
	}
}

// An edit stretches the block it is typed in; one taking the break between
// two blocks joins them, the first's.
func TestEditsStretchAndJoinBlocks(t *testing.T) {
	flow := textFlow(t, "ab\ncd\nef")
	blocks := func(preview *Frame) []model.UTF16Range {
		var ranges []model.UTF16Range
		for _, block := range preview.Flow().Blocks() {
			ranges = append(ranges, block.Range)
		}
		return ranges
	}
	typed := blocks(previewFrame(t, flow, model.EmptyUTF16Range(2), "xy"))
	if len(typed) != 3 || typed[0] != model.NewUTF16Range(0, 4) || typed[1] != model.NewUTF16Range(5, 7) {
		t.Fatalf("typing at a block's end gives %+v", typed)
	}
	joined := blocks(previewFrame(t, flow, model.NewUTF16Range(1, 4), "-"))
	if len(joined) != 2 || joined[0] != model.NewUTF16Range(0, 3) || joined[1] != model.NewUTF16Range(4, 6) {
		t.Fatalf("an edit over a break gives %+v", joined)
	}
}

// Objects are one list, found by where they stand and by their kind; a
// formula typed in moves with its source.
func TestObjectsAreOneList(t *testing.T) {
	flow, err := ownedBy(textFlow(t, "- [ ] a $x$\n￼"), []Object{
		Checkbox{Node: 1, Marker: model.NewUTF16Range(0, 5)},
		Formula{Node: 1, Range: model.NewUTF16Range(8, 11), Text: model.NewUTF16Range(9, 10), Source: "x"},
		Figure{Node: 2, Range: model.NewUTF16Range(12, 13), Width: 1, Height: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := flow.ObjectsIn(model.NewUTF16Range(4, 9)); len(got) != 2 {
		t.Fatalf("objects over 4..9 = %+v", got)
	}
	if got := ObjectsOf[Formula](flow); len(got) != 1 || got[0].Source != "x" {
		t.Fatalf("formulas = %+v", got)
	}
	if _, ok := ObjectOf[Figure](flow, 12); !ok {
		t.Fatal("no figure at its object")
	}
	if _, ok := ObjectOf[Figure](flow, 13); ok {
		t.Fatal("a figure past its object")
	}
	typed := ObjectsOf[Formula](previewFrame(t, flow, model.EmptyUTF16Range(10), "+y").Flow())
	if len(typed) != 1 || typed[0].Range != model.NewUTF16Range(8, 13) || typed[0].Text != model.NewUTF16Range(9, 12) {
		t.Fatalf("a formula typed in = %+v", typed)
	}
	if _, err := flow.WithObjects([]Object{Figure{Node: 9, Range: model.NewUTF16Range(12, 13), Width: 1, Height: 1}}); err == nil {
		t.Fatal("an object of another block was taken")
	}
}

package surface

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func TestCheckboxMetadataIsOwnedAndMappedThroughPreview(t *testing.T) {
	checks := []Checkbox{{Node: 2, Marker: model.NewUTF16Range(2, 7), Checked: true, Label: "task"}}
	flow, err := textFlow(t, "a\n- [x] task").WithCheckboxes(checks)
	if err != nil {
		t.Fatal(err)
	}
	checks[0].Node = 99
	copy := flow.Checkboxes()
	copy[0].Node = 88
	if flow.Checkboxes()[0].Node != 2 {
		t.Fatal("checkbox metadata is not owned")
	}
	before := previewFrame(t, flow, model.EmptyUTF16Range(1), "🦀").Flow().Checkboxes()
	if len(before) != 1 || before[0].Marker != model.NewUTF16Range(4, 9) || !before[0].Checked {
		t.Fatalf("before marker: %+v", before)
	}
	after := previewFrame(t, flow, model.EmptyUTF16Range(8), "字").Flow().Checkboxes()
	if len(after) != 1 || after[0].Marker != model.NewUTF16Range(2, 7) {
		t.Fatalf("after marker: %+v", after)
	}
	inside := previewFrame(t, flow, model.EmptyUTF16Range(4), "字").Flow().Checkboxes()
	if len(inside) != 0 {
		t.Fatalf("overwritten marker retained: %+v", inside)
	}
}

func TestCheckboxMetadataRejectsInvalidRanges(t *testing.T) {
	flow := textFlow(t, "abc")
	for _, checks := range [][]Checkbox{
		{{Node: 0, Marker: model.NewUTF16Range(0, 1)}},
		{{Node: 1, Marker: model.NewUTF16Range(1, 1)}},
		{{Node: 1, Marker: model.NewUTF16Range(1, 4)}},
		{{Node: 1, Marker: model.NewUTF16Range(1, 3)}, {Node: 2, Marker: model.NewUTF16Range(2, 3)}},
	} {
		if _, err := flow.WithCheckboxes(checks); err == nil {
			t.Fatalf("accepted %+v", checks)
		}
	}
}

package surface

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

// A frame says which of its flow's objects are edited within, by their
// runs: only the flow's own, in order. A composition in one's source keeps
// it so, stretched with what is composed, one before it moves it, and one
// that takes it away leaves nothing edited.
func TestEditingFollowsTheComposition(t *testing.T) {
	f := Formula{Node: 1, Range: model.NewUTF16Range(2, 5), Text: model.NewUTF16Range(3, 4), Source: "x"}
	flow, err := ownedBy(textFlow(t, "a $x$ b"), asObjects([]Formula{f}))
	if err != nil {
		t.Fatal(err)
	}
	frame, err := NewFrame(flow, model.EmptyUTF16Range(4), 4, TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, runs := range [][]model.UTF16Range{{model.NewUTF16Range(2, 4)}, {f.Range, f.Range}} {
		if _, err := frame.WithEditing(runs); err == nil {
			t.Fatalf("editing %v was taken", runs)
		}
	}
	edited, err := frame.WithEditing([]model.UTF16Range{f.Range})
	if err != nil {
		t.Fatal(err)
	}
	preview := func(original model.UTF16Range, text string) []model.UTF16Range {
		t.Helper()
		next, err := edited.WithPreview(original, text, model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(text))))
		if err != nil {
			t.Fatal(err)
		}
		return next.Editing()
	}
	if got := preview(model.EmptyUTF16Range(4), "²"); len(got) != 1 || got[0] != model.NewUTF16Range(2, 6) {
		t.Fatalf("composing in the source edits %v", got)
	}
	if got := preview(model.EmptyUTF16Range(0), "🦀"); len(got) != 1 || got[0] != model.NewUTF16Range(4, 7) {
		t.Fatalf("composing before the formula edits %v", got)
	}
	if got := preview(model.NewUTF16Range(1, 3), "q"); len(got) != 0 {
		t.Fatalf("composing over its notation still edits %v", got)
	}
	if got := frame.Editing(); len(got) != 0 {
		t.Fatalf("a frame no one edits in edits %v", got)
	}
}

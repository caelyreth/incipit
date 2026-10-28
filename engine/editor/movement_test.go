package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func TestTextMovementKeepsGraphemesWhole(t *testing.T) {
	t.Parallel()
	text := "a\u0301👩‍👩‍👧‍👦🇨🇳z"
	offsets := []model.UTF16Offset{0, 2, 13, 17, 18}
	head := model.UTF16Offset(0)
	for _, want := range offsets[1:] {
		plan, err := PlanTextCommand(text, head, head, TextRight)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Head != want {
			t.Fatalf("right from %d = %d, want %d", head, plan.Head, want)
		}
		head = plan.Head
	}
	for index := len(offsets) - 2; index >= 0; index-- {
		want := offsets[index]
		plan, err := PlanTextCommand(text, head, head, TextLeft)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Head != want {
			t.Fatalf("left from %d = %d, want %d", head, plan.Head, want)
		}
		head = plan.Head
	}
}

func TestScalarCaretInsideClusterDeletesWholeCluster(t *testing.T) {
	t.Parallel()
	text := "🇨🇳🇺🇸z"
	for _, command := range []TextCommand{TextBackspace, TextDelete} {
		plan, err := PlanTextCommand(text, 2, 2, command)
		if err != nil {
			t.Fatal(err)
		}
		if !plan.Delete || plan.DeleteAt != model.NewUTF16Range(0, 4) {
			t.Fatalf("%v plan = %#v", command, plan)
		}
	}
}

func TestDirectionCollapseLineAndDocumentCommands(t *testing.T) {
	t.Parallel()
	text := "ab\n🦀中\n"
	plan, err := PlanTextCommand(text, 7, 5, TextLeft)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Anchor != 5 || plan.Head != 5 {
		t.Fatalf("collapse = %#v", plan)
	}
	plan, _ = PlanTextCommand(text, 7, 5, TextLineStart)
	if plan.Head != 3 {
		t.Fatalf("line start = %#v", plan)
	}
	plan, _ = PlanTextCommand(text, 3, 3, TextLineEnd)
	if plan.Head != 6 {
		t.Fatalf("line end = %#v", plan)
	}
	plan, _ = PlanTextCommand(text, 0, 0, TextSelectAll)
	if plan.Anchor != 0 || plan.Head != 7 {
		t.Fatalf("select all = %#v", plan)
	}
	plan, _ = PlanTextCommand(text, 2, 2, TextNewline)
	if plan.Replace == nil || *plan.Replace != "\n" || plan.DeleteAt != model.NewUTF16Range(2, 2) {
		t.Fatalf("newline = %#v", plan)
	}
}

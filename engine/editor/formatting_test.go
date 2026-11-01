package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func TestFormattingLevelUsesStrongestCoveringMark(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abcdef")
	if _, err := fixture.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(fixture.node, model.NewUTF16Range(0, 4), EmphasisMark, 1); err != nil {
			return err
		}
		_, err := draft.AddMark(fixture.node, model.NewUTF16Range(1, 3), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	node := fixture.nodeValue(t)
	if got := FormattingLevel(node, fixture.selection(t, 1, 3), nil, true); got != 2 {
		t.Fatalf("covered strong level=%d", got)
	}
	if got := FormattingLevel(node, fixture.selection(t, 0, 2), nil, true); got != 1 {
		t.Fatalf("outer level=%d", got)
	}
	if got := FormattingLevel(node, fixture.selection(t, 4, 5), nil, true); got != 0 {
		t.Fatalf("uncovered level=%d", got)
	}
	if got := FormattingLevel(node, fixture.selection(t, 1, 3), nil, false); got != 0 {
		t.Fatalf("disabled level=%d", got)
	}
}

func TestFormattingLevelUsesActiveMarkForCollapsedSelection(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abc", model.NewUTF16Range(0, 3))
	node := fixture.nodeValue(t)
	mark := node.Marks()[0].ID()
	if got := FormattingLevel(node, fixture.selection(t, 3, 3), &mark, true); got != 1 {
		t.Fatalf("active mark level=%d", got)
	}
	other := model.MarkID(99)
	if got := FormattingLevel(node, fixture.selection(t, 3, 3), &other, true); got != 0 {
		t.Fatalf("inactive collapsed level=%d", got)
	}
}

func TestFormattingLevelTreatsAdjacentMarksAsCoverageWithoutMergingThem(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t,
		"abcd",
		model.NewUTF16Range(0, 2),
		model.NewUTF16Range(2, 4),
	)
	node := fixture.nodeValue(t)
	if node.MarkCount() != 2 {
		t.Fatalf("adjacent fixture marks = %#v", node.Marks())
	}
	if got := FormattingLevel(node, fixture.selection(t, 0, 4), nil, true); got != 1 {
		t.Fatalf("adjacent coverage level = %d", got)
	}
	settings := MarkupSettings{PolicyEnabled: true, CommandEnabled: true}
	if state := MarkupStateFor(MarkupEmphasis, settings, node, fixture.selection(t, 0, 4), nil); !state.Selected {
		t.Fatalf("adjacent emphasis state = %#v", state)
	}
	if node.MarkCount() != 2 {
		t.Fatal("coverage merged adjacent marks")
	}
}

func TestFormattingLevelPreservesNestedAndMixedCoverageMeaning(t *testing.T) {
	t.Parallel()
	nested := newEditingFixture(t, "abcd")
	if _, err := nested.document.Transact(func(draft *model.DocumentDraft) error {
		if _, err := draft.AddMark(nested.node, model.NewUTF16Range(0, 4), EmphasisMark, 1); err != nil {
			return err
		}
		_, err := draft.AddMark(nested.node, model.NewUTF16Range(1, 3), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	node := nested.nodeValue(t)
	if got := FormattingLevel(node, nested.selection(t, 1, 3), nil, true); got != 2 {
		t.Fatalf("nested inner level = %d", got)
	}
	if got := FormattingLevel(node, nested.selection(t, 0, 2), nil, true); got != 1 {
		t.Fatalf("nested outer level = %d", got)
	}

	mixed := newEditingFixture(t,
		"abcd",
		model.NewUTF16Range(0, 2),
	)
	if _, err := mixed.document.Transact(func(draft *model.DocumentDraft) error {
		_, err := draft.AddMark(mixed.node, model.NewUTF16Range(2, 4), EmphasisMark, 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	mixedNode := mixed.nodeValue(t)
	if got := FormattingLevel(mixedNode, mixed.selection(t, 0, 4), nil, true); got != 0 {
		t.Fatalf("mixed adjacent level = %d, want no single effective level", got)
	}
	if got := FormattingLevel(mixedNode, mixed.selection(t, 0, 3), nil, true); got != 0 {
		t.Fatalf("partially covered level = %d", got)
	}
}

func TestMarkupStateStrongAndEmphasis(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "abc", model.NewUTF16Range(0, 3))
	node := fixture.nodeValue(t)
	mark := node.Marks()[0].ID()
	settings := MarkupSettings{PolicyEnabled: true, CommandEnabled: true}
	if state := MarkupStateFor(MarkupStrong, settings, node, fixture.selection(t, 0, 3), &mark); !state.Enabled || state.Selected {
		t.Fatalf("strong state=%#v", state)
	}
	if state := MarkupStateFor(MarkupEmphasis, settings, node, fixture.selection(t, 0, 3), &mark); !state.Enabled || !state.Selected {
		t.Fatalf("emphasis state=%#v", state)
	}
	settings.PolicyEnabled = false
	if state := MarkupStateFor(MarkupEmphasis, settings, node, fixture.selection(t, 0, 3), &mark); state.Enabled {
		t.Fatalf("disabled state=%#v", state)
	}
}

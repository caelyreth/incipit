package input

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func inputDocument(t *testing.T, text string) (*model.Document, model.NodeID) {
	t.Helper()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(node, text)
	}); err != nil {
		t.Fatal(err)
	}
	return document, node
}

func TestTextTargetsCaptureAnchorsAndRebaseInEditOrder(t *testing.T) {
	document, node := inputDocument(t, "ab🦀cd")
	snapshot := document.Snapshot()
	target, err := TextTargetFromAnchors(
		model.NewTextAnchor(node, 2, model.AffinityLeft, snapshot.Revision()),
		model.NewTextAnchor(node, 4, model.AffinityRight, snapshot.Revision()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if target.Node() != node || target.RangeUTF16() != model.NewUTF16Range(2, 4) {
		t.Fatalf("target = %#v", target)
	}
	edits := []model.TextEdit{
		model.NewTextEditReplacement(node, model.NewUTF16Range(0, 0), "界"),
		model.NewTextEditReplacement(node, model.NewUTF16Range(3, 5), "x"),
	}
	rebased, ok := target.Rebase(edits, snapshot.Revision()+2)
	if !ok {
		t.Fatal("target failed to rebase")
	}
	if rebased.RangeUTF16() != model.NewUTF16Range(3, 4) ||
		rebased.Revision() != snapshot.Revision()+2 {
		t.Fatalf("rebased = %#v", rebased)
	}
}

func TestCollapsedTargetAtInsertionKeepsPositionalAffinity(t *testing.T) {
	t.Parallel()
	const node model.NodeID = 1
	target := NewTextTarget(node, model.EmptyUTF16Range(2), 7).
		WithAffinity(model.AffinityLeft).
		WithBoundaryAffinity(model.AffinityLeft, model.AffinityRight)
	if target.StartAffinity() != model.AffinityLeft ||
		target.EndAffinity() != model.AffinityRight {
		t.Fatalf("editing affinities changed: %#v", target)
	}
	if target.StartPositionAffinity() != model.AffinityRight ||
		target.EndPositionAffinity() != model.AffinityRight {
		t.Fatalf("editing affinity overloaded as position: %#v", target)
	}
	insert := model.NewTextEditReplacement(node, model.EmptyUTF16Range(2), "🦀")
	defaultRebased, ok := target.Rebase([]model.TextEdit{insert}, 8)
	if !ok || defaultRebased.RangeUTF16() != model.EmptyUTF16Range(4) {
		t.Fatalf("default rebase = %#v, %v", defaultRebased, ok)
	}
	left := target.WithPositionAffinity(model.AffinityLeft)
	leftRebased, ok := left.Rebase([]model.TextEdit{insert}, 8)
	if !ok || leftRebased.RangeUTF16() != model.EmptyUTF16Range(2) {
		t.Fatalf("left rebase = %#v, %v", leftRebased, ok)
	}
	if leftRebased.StartAffinity() != model.AffinityLeft ||
		leftRebased.EndAffinity() != model.AffinityRight {
		t.Fatalf("rebased editing affinities = %#v", leftRebased)
	}
	fromAnchor := TextTargetFromAnchor(model.NewTextAnchor(
		node,
		2,
		model.AffinityLeft,
		7,
	))
	fromAnchor, ok = fromAnchor.Rebase([]model.TextEdit{insert}, 8)
	if !ok || fromAnchor.RangeUTF16() != model.EmptyUTF16Range(2) {
		t.Fatalf("anchor target rebase = %#v, %v", fromAnchor, ok)
	}
}

func TestCollapsedTargetCannotAcquireOpposingPositionAffinities(t *testing.T) {
	target := NewTextTarget(1, model.EmptyUTF16Range(2), 1).
		WithPositionBoundaryAffinity(model.AffinityLeft, model.AffinityRight)
	edit := model.NewTextEditReplacement(1, model.EmptyUTF16Range(2), "x")
	rebased, ok := target.Rebase([]model.TextEdit{edit}, 2)
	if !ok || rebased.RangeUTF16() != model.EmptyUTF16Range(3) {
		t.Fatalf("collapsed target rebased to %v, ok=%v", rebased.RangeUTF16(), ok)
	}
}

func TestTextTargetRebaseCoversEditsAndTransfers(t *testing.T) {
	t.Parallel()
	const (
		node  model.NodeID = 1
		other model.NodeID = 2
	)
	tests := []struct {
		name      string
		edit      model.TextEdit
		want      model.NodeID
		collapsed model.UTF16Offset
		nonempty  model.UTF16Range
	}{
		{
			name:      "insertion before",
			edit:      model.NewTextEditReplacement(node, model.EmptyUTF16Range(1), "🦀"),
			want:      node,
			collapsed: 4,
			nonempty:  model.NewUTF16Range(4, 6),
		},
		{
			name:      "deletion before",
			edit:      model.NewTextEditReplacement(node, model.NewUTF16Range(1, 2), ""),
			want:      node,
			collapsed: 1,
			nonempty:  model.NewUTF16Range(1, 3),
		},
		{
			name:      "replacement before",
			edit:      model.NewTextEditReplacement(node, model.NewUTF16Range(0, 1), "界"),
			want:      node,
			collapsed: 2,
			nonempty:  model.NewUTF16Range(2, 4),
		},
		{
			name:      "deletion covers target",
			edit:      model.NewTextEditReplacement(node, model.NewUTF16Range(1, 4), ""),
			want:      node,
			collapsed: 1,
			nonempty:  model.NewUTF16Range(1, 1),
		},
		{
			name:      "replacement covers target",
			edit:      model.NewTextEditReplacement(node, model.NewUTF16Range(1, 4), "x"),
			want:      node,
			collapsed: 2,
			nonempty:  model.NewUTF16Range(1, 2),
		},
		{
			name:      "transfer",
			edit:      model.NewTextTransfer(node, 1, other, 0, false),
			want:      other,
			collapsed: 1,
			nonempty:  model.NewUTF16Range(1, 3),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			collapsed := NewTextTarget(node, model.EmptyUTF16Range(2), 3)
			collapsed, ok := collapsed.Rebase([]model.TextEdit{test.edit}, 4)
			if !ok || collapsed.Node() != test.want ||
				collapsed.RangeUTF16() != model.EmptyUTF16Range(test.collapsed) {
				t.Fatalf("collapsed = %#v, %v", collapsed, ok)
			}
			nonempty := NewTextTarget(node, model.NewUTF16Range(2, 4), 3)
			nonempty, ok = nonempty.Rebase([]model.TextEdit{test.edit}, 4)
			if !ok || nonempty.Node() != test.want ||
				nonempty.RangeUTF16() != test.nonempty {
				t.Fatalf("nonempty = %#v, %v", nonempty, ok)
			}
		})
	}
}

func TestCollapsedTargetAtTransferUsesPositionalAffinity(t *testing.T) {
	t.Parallel()
	const (
		node  model.NodeID = 1
		other model.NodeID = 2
	)
	transfer := model.NewTextTransfer(node, 2, other, 0, false)
	right := NewTextTarget(node, model.EmptyUTF16Range(2), 3)
	right, ok := right.Rebase([]model.TextEdit{transfer}, 4)
	if !ok || right.Node() != other || right.RangeUTF16() != model.EmptyUTF16Range(0) {
		t.Fatalf("right transfer = %#v, %v", right, ok)
	}
	left := NewTextTarget(node, model.EmptyUTF16Range(2), 3).
		WithPositionAffinity(model.AffinityLeft)
	left, ok = left.Rebase([]model.TextEdit{transfer}, 4)
	if !ok || left.Node() != node || left.RangeUTF16() != model.EmptyUTF16Range(2) {
		t.Fatalf("left transfer = %#v, %v", left, ok)
	}
}

func TestTargetValidationRejectsStaleMissingAndSplitScalar(t *testing.T) {
	document, node := inputDocument(t, "a🦀b")
	snapshot := document.Snapshot()
	if err := NewTextTarget(node, model.NewUTF16Range(0, 3), snapshot.Revision()).
		Validate(snapshot.Revision(), snapshot); err != nil {
		t.Fatal(err)
	}
	err := NewTextTarget(node, model.NewUTF16Range(0, 1), snapshot.Revision()).
		Validate(snapshot.Revision()+1, snapshot)
	var inputError *Error
	if !errors.As(err, &inputError) || inputError.Kind != ErrorStaleTarget {
		t.Fatalf("stale error = %v", err)
	}
	err = NewTextTarget(node, model.NewUTF16Range(1, 2), snapshot.Revision()).
		Validate(snapshot.Revision(), snapshot)
	if !errors.As(err, &inputError) || inputError.Kind != ErrorDocument {
		t.Fatalf("split scalar error = %v", err)
	}
	err = NewTextTarget(model.NodeID(999), model.NewUTF16Range(0, 0), snapshot.Revision()).
		Validate(snapshot.Revision(), snapshot)
	if !errors.As(err, &inputError) || inputError.Kind != ErrorDocument {
		t.Fatalf("missing node error = %v", err)
	}
}

func TestAttributeTargetRejectsEmptyKeyAndMissingNode(t *testing.T) {
	document, node := inputDocument(t, "abc")
	snapshot := document.Snapshot()
	valid := NewAttributeTarget(node, "emphasis/1", snapshot.Revision())
	if err := valid.Validate(snapshot.Revision(), snapshot); err != nil {
		t.Fatal(err)
	}
	for _, target := range []AttributeTarget{
		NewAttributeTarget(node, "", snapshot.Revision()),
		NewAttributeTarget(model.NodeID(999), "emphasis/1", snapshot.Revision()),
	} {
		err := target.Validate(snapshot.Revision(), snapshot)
		var inputError *Error
		if !errors.As(err, &inputError) || inputError.Kind != ErrorDocument {
			t.Fatalf("target %#v error = %v", target, err)
		}
	}
}

func TestDraftValidationRejectsWritesBeforeTargetCheck(t *testing.T) {
	document, node := inputDocument(t, "abc")
	target := NewTextTarget(node, model.NewUTF16Range(0, 1), document.Revision())
	draft := document.BeginDraft()
	if err := ValidateTextTargetDraft(target, document.Revision(), draft); err != nil {
		t.Fatal(err)
	}
	if err := draft.SetAttribute(node, "touched", model.BoolValue(true)); err != nil {
		t.Fatal(err)
	}
	err := ValidateTextTargetDraft(target, document.Revision(), draft)
	var inputError *Error
	if !errors.As(err, &inputError) || inputError.Kind != ErrorModifiedDocument {
		t.Fatalf("modified draft error = %v", err)
	}
}

func TestInputValuesRetainTargetsAndExposeDiagnostics(t *testing.T) {
	_, node := inputDocument(t, "abc")
	textTarget := NewTextTarget(node, model.NewUTF16Range(1, 2), 4)
	text := NewTextInput(textTarget, "x")
	if text.Target() != textTarget || text.Text() != "x" {
		t.Fatalf("text input = %#v", text)
	}
	attributeTarget := NewAttributeTarget(node, "emphasis/1", 4)
	attribute := NewAttributeInput(attributeTarget, "**")
	if attribute.Target() != attributeTarget || attribute.Text() != "**" {
		t.Fatalf("attribute input = %#v", attribute)
	}
	if got := attribute.String(); got == "" {
		t.Fatal("attribute diagnostic is empty")
	}
}

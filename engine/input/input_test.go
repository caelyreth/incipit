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

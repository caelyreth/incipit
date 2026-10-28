package model

import (
	"errors"
	"testing"
)

func TestAffinityKeepsInsertedTextOnRequestedSide(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	edit := NewTextEditReplacement(node, NewUTF16Range(2, 2), "界🐻")
	left := NewTextAnchor(node, 2, AffinityLeft, 1)
	right := NewTextAnchor(node, 2, AffinityRight, 1)
	gotLeft, ok := left.Rebase(edit, 2)
	if !ok || gotLeft.OffsetUTF16() != 2 {
		t.Fatalf("left = %#v,%v", gotLeft, ok)
	}
	gotRight, ok := right.Rebase(edit, 2)
	if !ok || gotRight.OffsetUTF16() != 5 {
		t.Fatalf("right = %#v,%v", gotRight, ok)
	}
}

func TestReplacementCollapsesInsideAnchorsAndShiftsFollowingAnchors(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	edit := NewTextEditReplacement(node, NewUTF16Range(2, 6), "x")
	for _, offset := range []UTF16Offset{2, 3, 5, 6} {
		left, ok := NewTextAnchor(node, offset, AffinityLeft, 1).Rebase(edit, 2)
		if !ok || left.OffsetUTF16() != 2 {
			t.Fatalf("left at %d = %#v,%v", offset, left, ok)
		}
		right, ok := NewTextAnchor(node, offset, AffinityRight, 1).Rebase(edit, 2)
		if !ok || right.OffsetUTF16() != 3 {
			t.Fatalf("right at %d = %#v,%v", offset, right, ok)
		}
	}
	after, ok := NewTextAnchor(node, 8, AffinityLeft, 1).Rebase(edit, 2)
	if !ok || after.OffsetUTF16() != 5 {
		t.Fatalf("after = %#v,%v", after, ok)
	}
}

func TestUnrelatedNodesAreUnchangedButRevisionAdvances(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	var other NodeID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		other, err = draft.CreateNode(node, 0, "paragraph")
		return err
	})
	edit := NewTextEditReplacement(node, NewUTF16Range(0, 0), "x")
	anchor, ok := NewTextAnchor(other, 4, AffinityRight, 1).Rebase(edit, 2)
	if !ok || anchor.Node() != other || anchor.OffsetUTF16() != 4 || anchor.Revision() != 2 {
		t.Fatalf("anchor = %#v,%v", anchor, ok)
	}
}

func TestRangesRequireMatchingNodeAndRevision(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	first := NewTextAnchor(node, 1, AffinityLeft, 4)
	second := NewTextAnchor(node, 3, AffinityRight, 4)
	value, err := first.RangeTo(second)
	if err != nil || value != NewUTF16Range(1, 3) {
		t.Fatalf("range = %#v,%v", value, err)
	}
	otherRevision := NewTextAnchor(node, 4, AffinityRight, 5)
	_, err = first.RangeTo(otherRevision)
	var anchorError *AnchorError
	if !errors.As(err, &anchorError) || anchorError.Kind != AnchorErrorRevisionMismatch {
		t.Fatalf("revision mismatch = %v", err)
	}
	otherNode := NewTextAnchor(node+1, 2, AffinityRight, 4)
	_, err = first.RangeTo(otherNode)
	if !errors.As(err, &anchorError) || anchorError.Kind != AnchorErrorDifferentNode {
		t.Fatalf("node mismatch = %v", err)
	}
}

func TestSelectionsKeepDirectionButExposeNormalizedRange(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	anchor := NewTextAnchor(node, 6, AffinityLeft, 3)
	head := NewTextAnchor(node, 2, AffinityRight, 3)
	selection := NewTextSelection(anchor, head)
	if selection.IsCollapsed() {
		t.Fatal("non-empty selection reported collapsed")
	}
	value, err := selection.Range()
	if err != nil || value != NewUTF16Range(2, 6) {
		t.Fatalf("range = %#v,%v", value, err)
	}
	if selection.Anchor() != anchor || selection.Head() != head {
		t.Fatal("selection direction was lost")
	}
}

func TestSelectionsRebaseBothEndpointsAndValidateSnapshots(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(node, "ab🦀cd")
	})
	before := document.Snapshot()
	selection := NewTextSelection(
		NewTextAnchor(node, 2, AffinityLeft, before.Revision()),
		NewTextAnchor(node, 4, AffinityRight, before.Revision()),
	)
	if err := selection.ValidateDocument(before); err != nil {
		t.Fatal(err)
	}
	edit := NewTextEditReplacement(node, NewUTF16Range(2, 4), "界")
	after, ok := selection.Rebase(edit, before.Revision()+1)
	if !ok {
		t.Fatal("selection failed to rebase")
	}
	value, err := after.Range()
	if err != nil || value != NewUTF16Range(2, 3) {
		t.Fatalf("range = %#v,%v", value, err)
	}
	if err := after.ValidateDocument(before); err == nil {
		t.Fatal("stale selection validated")
	}
}

func TestSelectionsRebaseThroughMultipleEditsInOrder(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	node := document.Snapshot().Root()
	selection := NewTextSelection(
		NewTextAnchor(node, 2, AffinityLeft, 7),
		NewTextAnchor(node, 4, AffinityRight, 7),
	)
	edits := []TextEdit{
		NewTextEditReplacement(node, NewUTF16Range(0, 0), "界"),
		NewTextEditReplacement(node, NewUTF16Range(3, 5), "🐻"),
	}
	rebased, ok := selection.RebaseAll(edits, 8)
	if !ok {
		t.Fatal("selection failed to rebase")
	}
	value, err := rebased.Range()
	if err != nil || value != NewUTF16Range(3, 5) {
		t.Fatalf("range = %#v,%v", value, err)
	}
	if rebased.Revision() != 8 {
		t.Fatalf("revision = %d", rebased.Revision())
	}
}

func TestEmptyFootprintAdvancesSelectionRevision(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	selection := NewTextSelection(
		NewTextAnchor(node, 3, AffinityRight, 4),
		NewTextAnchor(node, 1, AffinityLeft, 4),
	)
	rebased, ok := selection.RebaseAll(nil, 5)
	if !ok {
		t.Fatal("empty footprint failed")
	}
	if rebased.Anchor().Revision() != 5 ||
		rebased.Head().Revision() != 5 ||
		rebased.Anchor().OffsetUTF16() != 3 ||
		rebased.Head().OffsetUTF16() != 1 {
		t.Fatalf("rebased = %#v", rebased)
	}
}

func TestAnchorValidationRejectsStaleMissingAndSplitSurrogate(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(node, "a🦀b")
	})
	snapshot := document.Snapshot()
	if err := NewTextAnchor(node, 1, AffinityLeft, snapshot.Revision()).ValidateDocument(snapshot); err != nil {
		t.Fatal(err)
	}
	err := NewTextAnchor(node, 2, AffinityLeft, snapshot.Revision()).ValidateDocument(snapshot)
	var anchorError *AnchorError
	if !errors.As(err, &anchorError) || anchorError.Kind != AnchorErrorDocument {
		t.Fatalf("split surrogate = %v", err)
	}
	err = NewTextAnchor(node, 0, AffinityLeft, snapshot.Revision()-1).ValidateDocument(snapshot)
	if !errors.As(err, &anchorError) || anchorError.Kind != AnchorErrorStale {
		t.Fatalf("stale = %v", err)
	}
	err = NewTextAnchor(node+1, 0, AffinityLeft, snapshot.Revision()).ValidateDocument(snapshot)
	if !errors.As(err, &anchorError) || anchorError.Kind != AnchorErrorDocument {
		t.Fatalf("missing node = %v", err)
	}
}

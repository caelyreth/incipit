package model

import (
	"errors"
	"slices"
	"testing"
)

func markRanges(marks []InlineMark) []UTF16Range {
	ranges := make([]UTF16Range, len(marks))
	for index, mark := range marks {
		ranges[index] = mark.Range()
	}
	return ranges
}

func TestSemanticMarksValidateRebaseAndRollbackWithText(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	var mark MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "a🦀b"); err != nil {
			return err
		}
		var err error
		mark, err = draft.AddMark(node, NewUTF16Range(1, 3), "emphasis", 1)
		return err
	})
	before := document.Snapshot()
	_, err := document.Transact(func(draft *DocumentDraft) error {
		_, err := draft.AddMark(node, NewUTF16Range(2, 3), "emphasis", 2)
		return err
	})
	if err == nil {
		t.Fatal("crossing mark accepted")
	}
	if document.Snapshot() != before {
		t.Fatal("failed crossing mark was published")
	}
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextMarked(node, 3, 3, "中", &mark)
	})
	snapshot := document.Snapshot()
	updated, _ := snapshot.Node(node)
	marks := updated.Marks()
	if len(marks) != 1 || marks[0].Range() != NewUTF16Range(1, 4) || marks[0].Level() != 1 {
		t.Fatalf("marks = %#v", marks)
	}
	if got := updated.Text().String(); got != "a🦀中b" {
		t.Fatalf("text = %q", got)
	}
}

func TestMarkAddExactRangeUpdatesLevelAndPreservesID(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	var first MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "word"); err != nil {
			return err
		}
		var err error
		first, err = draft.AddMark(node, NewUTF16Range(0, 4), "emphasis", 1)
		return err
	})
	var second MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		second, err = draft.AddMark(node, NewUTF16Range(0, 4), "emphasis", 2)
		return err
	})
	if first != second {
		t.Fatalf("same-range add changed ID: %d != %d", first, second)
	}
	nodeValue, _ := document.Snapshot().Node(node)
	marks := nodeValue.Marks()
	if len(marks) != 1 || marks[0].Level() != 2 {
		t.Fatalf("marks = %#v", marks)
	}
}

func TestNestedMarksOrderingAndValidation(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "a🦀b"); err != nil {
			return err
		}
		if _, err := draft.AddMark(node, NewUTF16Range(1, 3), "emphasis", 2); err != nil {
			return err
		}
		_, err := draft.AddMark(node, NewUTF16Range(0, 4), "emphasis", 1)
		return err
	})
	nodeValue, _ := document.Snapshot().Node(node)
	if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{
		NewUTF16Range(0, 4),
		NewUTF16Range(1, 3),
	}) {
		t.Fatalf("ranges = %#v", got)
	}
}

func TestMarkRebaseAtSharedBoundaryExpandsOnlyEnclosingMarks(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	var outer, inner MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "abc"); err != nil {
			return err
		}
		var err error
		outer, err = draft.AddMark(node, NewUTF16Range(0, 3), "emphasis", 1)
		if err != nil {
			return err
		}
		inner, err = draft.AddMark(node, NewUTF16Range(1, 3), "emphasis", 2)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextMarked(node, 3, 3, "x", &inner)
	})
	nodeValue, _ := document.Snapshot().Node(node)
	if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{
		NewUTF16Range(0, 4),
		NewUTF16Range(1, 4),
	}) {
		t.Fatalf("ranges = %#v", got)
	}
	if got := nodeValue.Text().String(); got != "abcx" {
		t.Fatalf("text = %q", got)
	}
	if outer == inner {
		t.Fatal("nested marks received same identity")
	}
}

func TestMarkRebaseWithoutActiveMarkExcludesBoundaryInsertion(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "abc"); err != nil {
			return err
		}
		_, err := draft.AddMark(node, NewUTF16Range(0, 3), "emphasis", 1)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextUTF16(node, 3, 3, "x")
	})
	nodeValue, _ := document.Snapshot().Node(node)
	if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{NewUTF16Range(0, 3)}) {
		t.Fatalf("ranges = %#v", got)
	}
}

func TestEmptyMarkRebaseUsesTheEditedBoundarySide(t *testing.T) {
	newDocument := func(t *testing.T) (*Document, NodeID) {
		t.Helper()
		document := mustDocument(t, "text")
		node := document.Snapshot().Root()
		mustTransact(t, document, func(draft *DocumentDraft) error {
			if err := draft.SetTextValue(node, "abc"); err != nil {
				return err
			}
			_, err := draft.AddMark(node, NewUTF16Range(3, 3), "emphasis", 1)
			return err
		})
		return document, node
	}

	t.Run("replacement before the mark keeps it after the text", func(t *testing.T) {
		document, node := newDocument(t)
		mustTransact(t, document, func(draft *DocumentDraft) error {
			return draft.ReplaceTextMarkedWithBoundaryAffinity(
				node, 2, 3, "y", nil, AffinityLeft, AffinityRight,
			)
		})
		nodeValue, _ := document.Snapshot().Node(node)
		if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{
			NewUTF16Range(3, 3),
		}) {
			t.Fatalf("mark ranges = %#v, want the mark at the new text end", got)
		}
	})

	t.Run("right affinity keeps an insertion before the mark", func(t *testing.T) {
		document, node := newDocument(t)
		mustTransact(t, document, func(draft *DocumentDraft) error {
			return draft.ReplaceTextMarkedWithBoundaryAffinity(
				node, 3, 3, "x", nil, AffinityRight, AffinityRight,
			)
		})
		nodeValue, _ := document.Snapshot().Node(node)
		if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{
			NewUTF16Range(4, 4),
		}) {
			t.Fatalf("mark ranges = %#v, want the mark after inserted text", got)
		}
	})

	t.Run("left affinity keeps an insertion after the mark", func(t *testing.T) {
		document, node := newDocument(t)
		mustTransact(t, document, func(draft *DocumentDraft) error {
			return draft.ReplaceTextMarkedWithBoundaryAffinity(
				node, 3, 3, "x", nil, AffinityLeft, AffinityLeft,
			)
		})
		nodeValue, _ := document.Snapshot().Node(node)
		if got := markRanges(nodeValue.Marks()); !slices.Equal(got, []UTF16Range{
			NewUTF16Range(3, 3),
		}) {
			t.Fatalf("mark ranges = %#v, want the mark before inserted text", got)
		}
	})
}

func TestMarkRebaseDeduplicatesRangesAtMaximumLevel(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "abcd"); err != nil {
			return err
		}
		if _, err := draft.AddMark(node, NewUTF16Range(0, 2), "emphasis", 1); err != nil {
			return err
		}
		if _, err := draft.AddMark(node, NewUTF16Range(2, 4), "emphasis", 2); err != nil {
			return err
		}
		return draft.ReplaceTextUTF16(node, 0, 4, "")
	})
	nodeValue, _ := document.Snapshot().Node(node)
	marks := nodeValue.Marks()
	if len(marks) != 1 || marks[0].Range() != NewUTF16Range(0, 0) || marks[0].Level() != 2 {
		t.Fatalf("deduplicated marks = %#v", marks)
	}
}

func TestMarkRebaseCanonicalizesActiveFirstMixedKindDuplicates(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	var active, otherMark, otherKind MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "ab"); err != nil {
			return err
		}
		var err error
		active, err = draft.AddMark(node, NewUTF16Range(0, 1), "z", 1)
		if err != nil {
			return err
		}
		otherMark, err = draft.AddMark(node, NewUTF16Range(1, 2), "z", 2)
		if err != nil {
			return err
		}
		otherKind, err = draft.AddMark(node, NewUTF16Range(0, 2), "a", 3)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextMarked(node, 0, 2, "", &active)
	})

	nodeValue, _ := document.Snapshot().Node(node)
	marks := nodeValue.Marks()
	if len(marks) != 2 {
		t.Fatalf("marks = %#v, want one z and one a", marks)
	}
	var z, a InlineMark
	for _, mark := range marks {
		switch mark.Kind() {
		case "z":
			z = mark
		case "a":
			a = mark
		}
	}
	if z.ID() != active || z.Level() != 2 || z.Range() != NewUTF16Range(0, 0) {
		t.Fatalf("z = %#v, want active id %d at maximum level", z, active)
	}
	if a.ID() != otherKind || a.Level() != 3 || a.Range() != NewUTF16Range(0, 0) {
		t.Fatalf("a = %#v, want independently kept id %d", a, otherKind)
	}
	if otherMark == active {
		t.Fatal("fixture reused a mark identity")
	}
}

func TestLegacyDuplicateMarksRetainIdentitiesOnRestore(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	snapshot := document.Snapshot()
	record := snapshot.Nodes()[0].Record()
	text := "a"
	record.Text = &text
	record.Marks = []InlineMark{
		NewInlineMark(1, NewUTF16Range(0, 1), "emphasis", 1),
		NewInlineMark(2, NewUTF16Range(0, 1), "emphasis", 2),
	}
	record.NextMarkID = 3
	restored, err := RestoreDocument(snapshot.Root(), snapshot.NextID(), []NodeRecord{record})
	if err != nil {
		t.Fatal(err)
	}
	node, _ := restored.Snapshot().Node(snapshot.Root())
	if got := node.Marks(); !slices.Equal(got, record.Marks) {
		t.Fatalf("legacy identities changed: got %+v, want %+v", got, record.Marks)
	}
	if _, err := restored.Transact(func(draft *DocumentDraft) error {
		return draft.SetAttribute(node.ID(), "note", TextValue("still editable"))
	}); err != nil {
		t.Fatalf("legacy document could not be edited: %v", err)
	}
}

func TestMarkAllocationExhaustionIsCheckedBeforeMutation(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		document := mustDocument(t, "text")
		node := document.Snapshot().Root()
		draft := document.BeginDraft()
		draft.state.nodes[node].nextMarkID = ^uint64(0)
		before := draft.state
		_, err := draft.AddMark(node, NewUTF16Range(0, 0), "emphasis", 1)
		var documentError *DocumentError
		if !errors.As(err, &documentError) ||
			documentError.Kind != DocumentErrorIdentityExhausted {
			t.Fatalf("error = %v", err)
		}
		if draft.state != before || draft.ChangeCount() != 0 || draft.TextEditCount() != 0 {
			t.Fatal("exhausted add mutated the draft")
		}
	})
	t.Run("update existing without an identity", func(t *testing.T) {
		document := mustDocument(t, "text")
		node := document.Snapshot().Root()
		mustTransact(t, document, func(draft *DocumentDraft) error {
			_, err := draft.AddMark(node, NewUTF16Range(0, 0), "emphasis", 1)
			return err
		})
		draft := document.BeginDraft()
		draft.state.nodes[node].nextMarkID = ^uint64(0)
		id, err := draft.AddMark(node, NewUTF16Range(0, 0), "emphasis", 2)
		if err != nil || id == 0 {
			t.Fatalf("update at exhausted identity supply: %v, %v", id, err)
		}
		updated, _ := draft.Node(node)
		if got := updated.Marks(); len(got) != 1 || got[0].ID() != id || got[0].Level() != 2 {
			t.Fatalf("updated marks = %+v", got)
		}
	})
	t.Run("join", func(t *testing.T) {
		document, ids := blockDocument(t, "a", "b")
		mustTransact(t, document, func(draft *DocumentDraft) error {
			_, err := draft.AddMark(ids[1], NewUTF16Range(0, 1), "emphasis", 1)
			return err
		})
		draft := document.BeginDraft()
		draft.state.nodes[ids[0]].nextMarkID = ^uint64(0)
		before := draft.state
		err := draft.JoinText(ids[0], ids[1])
		var documentError *DocumentError
		if !errors.As(err, &documentError) ||
			documentError.Kind != DocumentErrorIdentityExhausted {
			t.Fatalf("error = %v", err)
		}
		if draft.state != before || draft.ChangeCount() != 0 || draft.TextEditCount() != 0 {
			t.Fatal("exhausted join mutated the draft")
		}
	})
	t.Run("last available identity", func(t *testing.T) {
		document, ids := blockDocument(t, "a", "b")
		mustTransact(t, document, func(draft *DocumentDraft) error {
			_, err := draft.AddMark(ids[1], NewUTF16Range(0, 1), "emphasis", 1)
			return err
		})
		var joined MarkID
		mustTransact(t, document, func(draft *DocumentDraft) error {
			draft.state.nodes[ids[0]].nextMarkID = ^uint64(0) - 1
			if err := draft.JoinText(ids[0], ids[1]); err != nil {
				return err
			}
			joined = draft.state.nodes[ids[0]].marks[0].ID()
			return nil
		})
		left, _ := document.Snapshot().Node(ids[0])
		if joined != MarkID(^uint64(0)-1) || left.Marks()[0].ID() != joined {
			t.Fatalf("joined mark id = %d, marks = %#v", joined, left.Marks())
		}
	})
	t.Run("reservation covers every transferred mark", func(t *testing.T) {
		if err := checkNextMarkIDSupply(^uint64(0)-1, 1); err != nil {
			t.Fatalf("one remaining identity rejected: %v", err)
		}
		var documentError *DocumentError
		if err := checkNextMarkIDSupply(^uint64(0)-1, 2); !errors.As(err, &documentError) ||
			documentError.Kind != DocumentErrorIdentityExhausted {
			t.Fatalf("two identities beyond supply = %v", err)
		}
	})
}

func TestSetMarkLevelZeroRemovesAndIDsRemainMonotonicAfterRollback(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	var first MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(node, "word"); err != nil {
			return err
		}
		var err error
		first, err = draft.AddMark(node, NewUTF16Range(0, 2), "emphasis", 1)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetMarkLevel(node, first, 0)
	})
	var second MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		second, err = draft.AddMark(node, NewUTF16Range(0, 2), "emphasis", 1)
		return err
	})
	if second <= first {
		t.Fatalf("mark identity reused: %d after %d", second, first)
	}

	draft := document.BeginDraft()
	checkpoint := draft.Checkpoint()
	if err := draft.SetMarkLevel(node, second, 0); err != nil {
		t.Fatal(err)
	}
	draft.Rollback(checkpoint)
	nodeValue, _ := draft.Node(node)
	if nodeValue.MarkCount() != 1 {
		t.Fatal("rollback did not restore mark")
	}
}

func TestMarkStrengthAndKindValidate(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(node, "word")
	})
	draft := document.BeginDraft()
	draft.state.nodes[node].marks = []InlineMark{NewInlineMark(1, NewUTF16Range(0, 1), "", 1)}
	draft.state.nodes[node].nextMarkID = 2
	if err := draft.Validate(); err == nil {
		t.Fatal("a mark without a kind validated")
	}
}

func TestMarkOperationsRejectInvalidInputs(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	node := document.Snapshot().Root()
	draft := document.BeginDraft()
	if _, err := draft.AddMark(node, NewUTF16Range(0, 1), "emphasis", 1); err == nil {
		t.Fatal("out-of-range mark accepted")
	}
	if err := draft.SetTextValue(node, "abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := draft.AddMark(node, NewUTF16Range(0, 1), "emphasis", 0); err == nil {
		t.Fatal("zero-strength mark accepted")
	}
	mark, err := draft.AddMark(node, NewUTF16Range(0, 1), "emphasis", 1)
	if err != nil {
		t.Fatal(err)
	}
	// The model reads no meaning into strength: a policy caps it.
	if err := draft.SetMarkLevel(node, mark, 3); err != nil {
		t.Fatal(err)
	}
	if err := draft.SetMarkLevel(node, mark+100, 1); err == nil {
		t.Fatal("missing mark accepted")
	}
}

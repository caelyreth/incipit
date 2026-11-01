package model

import (
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

package model

import (
	"slices"
	"testing"
)

func blockDocument(t *testing.T, texts ...string) (*Document, []NodeID) {
	t.Helper()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	ids := make([]NodeID, 0, len(texts))
	mustTransact(t, document, func(draft *DocumentDraft) error {
		for index, text := range texts {
			id, err := draft.CreateNode(root, index, "paragraph")
			if err != nil {
				return err
			}
			if err := draft.SetTextValue(id, text); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	return document, ids
}

// Splitting moves the tail, its marks and its positions into a new block
// right after; joining brings them back.
func TestSplitAndJoinCarryTextMarksAndPositions(t *testing.T) {
	t.Parallel()
	document, ids := blockDocument(t, "one two", "three")
	first := ids[0]
	var second NodeID
	var edits []TextEdit
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if _, err := draft.AddMark(first, NewUTF16Range(2, 6), "emphasis", 1); err != nil {
			return err
		}
		if err := draft.SetAttribute(first, "level", IntegerValue(2)); err != nil {
			return err
		}
		var err error
		second, err = draft.SplitText(first, 4)
		edits = draft.TextEdits()
		return err
	})
	snapshot := document.Snapshot()
	root, _ := snapshot.Node(snapshot.Root())
	if !slices.Equal(root.Children(), []NodeID{first, second, ids[1]}) {
		t.Fatalf("children = %v", root.Children())
	}
	head, _ := snapshot.Node(first)
	tail, _ := snapshot.Node(second)
	if head.Text().String() != "one " || tail.Text().String() != "two" || tail.Kind() != "paragraph" {
		t.Fatalf("split %q / %q", head.Text().String(), tail.Text().String())
	}
	if level, _ := tail.Attribute("level"); level != IntegerValue(2) {
		t.Fatal("the new block lost its attributes")
	}
	if got := markRanges(head.Marks()); !slices.Equal(got, []UTF16Range{NewUTF16Range(2, 4)}) {
		t.Fatalf("head marks %v", got)
	}
	if got := markRanges(tail.Marks()); !slices.Equal(got, []UTF16Range{NewUTF16Range(0, 2)}) {
		t.Fatalf("tail marks %v", got)
	}
	stay := NewTextAnchor(first, 4, AffinityLeft, 0)
	move := NewTextAnchor(first, 4, AffinityRight, 0)
	deep := NewTextAnchor(first, 6, AffinityLeft, 0)
	for _, check := range []struct {
		anchor TextAnchor
		node   NodeID
		offset UTF16Offset
	}{{stay, first, 4}, {move, second, 0}, {deep, second, 2}} {
		got, ok := NewTextSelection(check.anchor, check.anchor).RebaseAll(edits, 9)
		if !ok || got.Head().Node() != check.node || got.Head().Offset() != check.offset {
			t.Fatalf("%v → %v", check.anchor, got.Head())
		}
	}

	var joined []TextEdit
	mustTransact(t, document, func(draft *DocumentDraft) error {
		err := draft.JoinText(first, second)
		joined = draft.TextEdits()
		return err
	})
	snapshot = document.Snapshot()
	head, _ = snapshot.Node(first)
	if _, exists := snapshot.Node(second); exists || head.Text().String() != "one two" {
		t.Fatalf("joined %q", head.Text().String())
	}
	if got := markRanges(head.Marks()); !slices.Equal(got, []UTF16Range{NewUTF16Range(2, 4), NewUTF16Range(4, 6)}) {
		t.Fatalf("joined marks %v", got)
	}
	if head.Marks()[0].ID() == head.Marks()[1].ID() {
		t.Fatal("adjacent joined fragments share one identity")
	}
	back, ok := NewTextSelection(NewTextAnchor(second, 0, AffinityLeft, 0), NewTextAnchor(second, 3, AffinityLeft, 0)).
		RebaseAll(joined, 10)
	if !ok || back.Anchor().Node() != first || back.Anchor().Offset() != 4 || back.Head().Offset() != 7 {
		t.Fatalf("join carried %v", back)
	}
}

func TestJoinCanonicalizesDuplicateBoundaryMarksAtMaximumLevel(t *testing.T) {
	t.Parallel()
	document, ids := blockDocument(t, "a", "b")
	var leftID MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		leftID, err = draft.AddMark(ids[0], NewUTF16Range(1, 1), "emphasis", 1)
		if err != nil {
			return err
		}
		_, err = draft.AddMark(ids[1], NewUTF16Range(0, 0), "emphasis", 2)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.JoinText(ids[0], ids[1])
	})
	joined, _ := document.Snapshot().Node(ids[0])
	marks := joined.Marks()
	if len(marks) != 1 || marks[0].Range() != NewUTF16Range(1, 1) ||
		marks[0].Level() != 2 || marks[0].ID() != leftID {
		t.Fatalf("joined marks = %#v, want left id %d at maximum level", marks, leftID)
	}
}

func TestSplitCanonicalizesMarksThatMeetAtTheBoundary(t *testing.T) {
	t.Parallel()
	document, ids := blockDocument(t, "ab")
	var outer MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		outer, err = draft.AddMark(ids[0], NewUTF16Range(0, 2), "emphasis", 1)
		if err != nil {
			return err
		}
		_, err = draft.AddMark(ids[0], NewUTF16Range(0, 1), "emphasis", 2)
		return err
	})
	var tail NodeID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		tail, err = draft.SplitText(ids[0], 1)
		return err
	})
	snapshot := document.Snapshot()
	head, _ := snapshot.Node(ids[0])
	tailNode, _ := snapshot.Node(tail)
	if marks := head.Marks(); len(marks) != 1 || marks[0].ID() != outer ||
		marks[0].Range() != NewUTF16Range(0, 1) || marks[0].Level() != 2 {
		t.Fatalf("head marks = %#v", marks)
	}
	if marks := tailNode.Marks(); len(marks) != 1 ||
		marks[0].Range() != NewUTF16Range(0, 1) || marks[0].Level() != 1 {
		t.Fatalf("tail marks = %#v", marks)
	}
}

func TestJoinKeepsIntentionalAdjacentMarksSeparate(t *testing.T) {
	t.Parallel()
	document, ids := blockDocument(t, "a", "b")
	var leftID, rightID MarkID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		leftID, err = draft.AddMark(ids[0], NewUTF16Range(0, 1), "emphasis", 1)
		if err != nil {
			return err
		}
		rightID, err = draft.AddMark(ids[1], NewUTF16Range(0, 1), "emphasis", 2)
		return err
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.JoinText(ids[0], ids[1])
	})
	joined, _ := document.Snapshot().Node(ids[0])
	marks := joined.Marks()
	if len(marks) != 2 ||
		marks[0].Range() != NewUTF16Range(0, 1) ||
		marks[1].Range() != NewUTF16Range(1, 2) {
		t.Fatalf("joined marks = %#v", marks)
	}
	if marks[0].ID() != leftID || marks[1].ID() == rightID || marks[0].ID() == marks[1].ID() {
		t.Fatalf("joined identities = %d, %d; right source = %d", marks[0].ID(), marks[1].ID(), rightID)
	}
}

// Restoring a snapshot brings back its blocks, kinds and order, and never
// rewinds identities.
func TestRestoreDocumentReturnsStructureWithoutRewindingIdentity(t *testing.T) {
	t.Parallel()
	document, ids := blockDocument(t, "alpha", "beta")
	before := document.Snapshot()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetNodeKind(ids[0], "heading"); err != nil {
			return err
		}
		if err := draft.JoinText(ids[0], ids[1]); err != nil {
			return err
		}
		_, err := draft.CreateNode(before.Root(), 1, "paragraph")
		return err
	})
	supply := document.Snapshot().state.nextID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.RestoreDocument(before)
	})
	after := document.Snapshot()
	root, _ := after.Node(after.Root())
	first, _ := after.Node(ids[0])
	if !slices.Equal(root.Children(), ids) || first.Kind() != "paragraph" || first.Text().String() != "alpha" {
		t.Fatalf("restored %v %q %q", root.Children(), first.Kind(), first.Text().String())
	}
	if after.state.nextID != supply {
		t.Fatal("restoring rewound node identities")
	}
}

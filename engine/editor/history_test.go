package editor

import (
	"errors"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

type fakeStateClock struct {
	now time.Time
}

func (c *fakeStateClock) Now() time.Time {
	return c.now
}

func (c *fakeStateClock) Add(duration time.Duration) {
	c.now = c.now.Add(duration)
}

func stateDocument(t *testing.T, text string) (*model.Document, model.NodeID) {
	t.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		t.Fatal(err)
	}
	root := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(root, text)
	}); err != nil {
		t.Fatal(err)
	}
	return document, root
}

func stateFrame(
	t *testing.T,
	document *model.Document,
	nodeID model.NodeID,
	offset model.UTF16Offset,
) Frame {
	t.Helper()
	snapshot := document.Snapshot()
	node, ok := snapshot.Node(nodeID)
	if !ok {
		t.Fatal("history fixture node missing")
	}
	anchor := model.NewTextAnchor(nodeID, offset, model.AffinityRight, snapshot.Revision())
	frame, ok := NewFrame(
		node,
		model.CollapsedSelection(anchor),
		projection.CollapsedSelection(projection.TextCaret(anchor)),
		nil,
	)
	if !ok {
		t.Fatal("history fixture frame rejected")
	}
	return frame
}

func stateVisual(revision uint64, offset model.UTF16Offset) projection.Selection {
	return projection.CollapsedSelection(projection.TextCaret(
		model.NewTextAnchor(1, offset, model.AffinityRight, revision),
	))
}

func TestHistoryGroupsContiguousTypingUpToTheExact750MillisecondBoundary(t *testing.T) {
	base := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		elapsed  time.Duration
		join     bool
		mismatch bool
	}{
		{name: "immediate", elapsed: 0, join: true},
		{name: "just below window", elapsed: 749 * time.Millisecond, join: true},
		{name: "exact window breaks", elapsed: 750 * time.Millisecond, join: false},
		{name: "beyond window breaks", elapsed: 751 * time.Millisecond, join: false},
		{name: "visual mismatch breaks", elapsed: 100 * time.Millisecond, join: false, mismatch: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock := &fakeStateClock{now: base}
			history := NewHistory(clock)
			document, root := stateDocument(t, "abc")
			first := stateFrame(t, document, root, 0)
			history.Record(first, stateVisual(first.selection.Revision(), 1), true, true)

			clock.Add(test.elapsed)
			before := first
			before.visual = stateVisual(first.selection.Revision(), 1)
			if test.mismatch {
				before.visual = stateVisual(first.selection.Revision(), 2)
			}
			history.Record(before, stateVisual(first.selection.Revision(), 2), true, true)

			wantDepth := 2
			if test.join {
				wantDepth = 1
			}
			if history.UndoDepth() != wantDepth {
				t.Fatalf("undo depth = %d, want %d", history.UndoDepth(), wantDepth)
			}
		})
	}
}

func TestHistoryOnlyGroupsSingleAlphanumericTypingInsertions(t *testing.T) {
	tests := []struct {
		name        string
		origin      kernel.EventOrigin
		appliesText bool
		replacement string
		group       bool
	}{
		{"ascii letter", kernel.EventOriginTyping, true, "a", true},
		{"unicode letter", kernel.EventOriginTyping, true, "中", true},
		{"digit", kernel.EventOriginTyping, true, "7", true},
		{"punctuation", kernel.EventOriginTyping, true, ",", false},
		{"combining scalar alone", kernel.EventOriginTyping, true, "\u0301", false},
		{"two scalars", kernel.EventOriginTyping, true, "ab", false},
		{"emoji", kernel.EventOriginTyping, true, "🦀", false},
		{"empty", kernel.EventOriginTyping, true, "", false},
		{"paste", kernel.EventOriginPaste, true, "a", false},
		{"ime", kernel.EventOriginIME, true, "a", false},
		{"semantic", kernel.EventOriginSemantic, true, "a", false},
		{"attribute only", kernel.EventOriginTyping, false, "a", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ShouldGroupTyping(test.origin, test.appliesText, test.replacement)
			if got != test.group {
				t.Fatalf("ShouldGroupTyping = %v, want %v", got, test.group)
			}
		})
	}
}

func TestHistoryBreakEventsPreventJoiningTheNextTypingEdit(t *testing.T) {
	tests := []struct {
		name       string
		breakGroup func(*History)
	}{
		{"selection", func(history *History) { history.SelectionChanged() }},
		{"configuration", func(history *History) { history.ConfigurationChanged() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clock := &fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
			history := NewHistory(clock)
			document, root := stateDocument(t, "abc")
			before := stateFrame(t, document, root, 0)
			history.Record(before, stateVisual(0, 1), true, true)
			test.breakGroup(history)
			clock.Add(100 * time.Millisecond)
			history.Record(before, stateVisual(0, 2), true, true)
			if history.UndoDepth() != 2 {
				t.Fatalf("undo depth = %d, want 2", history.UndoDepth())
			}
		})
	}
}

func TestHistoryUndoBreaksTypingGroupForTheNextEdit(t *testing.T) {
	clock := &fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	history := NewHistory(clock)
	document, root := stateDocument(t, "abc")
	before := stateFrame(t, document, root, 0)
	history.Record(before, stateVisual(0, 1), true, true)
	if _, err := history.Undo(before, func(Frame) error { return nil }); err != nil {
		t.Fatal(err)
	}
	clock.Add(100 * time.Millisecond)
	history.Record(before, stateVisual(0, 2), true, true)
	if history.UndoDepth() != 1 {
		t.Fatalf("undo depth = %d, want 1", history.UndoDepth())
	}
}

func TestHistoryNoOpBreaksGroupAndNeverRecords(t *testing.T) {
	clock := &fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	history := NewHistory(clock)
	document, root := stateDocument(t, "abc")
	before := stateFrame(t, document, root, 0)
	history.Record(before, stateVisual(0, 1), true, true)
	history.Record(before, stateVisual(0, 1), false, true)
	if history.UndoDepth() != 1 {
		t.Fatalf("no-op changed undo depth: %d", history.UndoDepth())
	}
	history.Record(before, stateVisual(0, 2), true, true)
	if history.UndoDepth() != 2 {
		t.Fatalf("typing joined across no-op: depth %d", history.UndoDepth())
	}
}

func TestHistoryUndoRedoStacksAndFailureAtomicity(t *testing.T) {
	history := NewHistory(&fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	document, root := stateDocument(t, "abc")
	before := stateFrame(t, document, root, 0)
	current := stateFrame(t, document, root, 1)
	history.Record(before, current.visual, true, false)

	var restored *Frame
	ok, err := history.Undo(current, func(frame Frame) error {
		restored = &frame
		return nil
	})
	if err != nil || !ok || restored == nil {
		t.Fatalf("undo = %v %v", ok, err)
	}
	if history.CanUndo() || !history.CanRedo() {
		t.Fatalf("undo/redo availability = %v/%v", history.CanUndo(), history.CanRedo())
	}

	ok, err = history.Redo(before, func(Frame) error { return nil })
	if err != nil || !ok || !history.CanUndo() || history.CanRedo() {
		t.Fatalf("redo = %v %v availability=%v/%v", ok, err, history.CanUndo(), history.CanRedo())
	}
	if history.UndoDepth() != 1 {
		t.Fatalf("redo depth = %d", history.UndoDepth())
	}

	ok, err = history.Undo(current, nil)
	if ok || !errors.Is(err, ErrHistoryRestoreRequired) {
		t.Fatalf("undo without callback = %v %v", ok, err)
	}

	sentinel := errors.New("restore failed")
	ok, err = history.Undo(current, func(Frame) error { return sentinel })
	if ok || !errors.Is(err, sentinel) {
		t.Fatalf("failed undo = %v %v", ok, err)
	}
	if !history.CanUndo() || history.CanRedo() {
		t.Fatal("failed undo moved a stack")
	}

	history.Record(before, current.visual, true, false)
	if history.CanRedo() {
		t.Fatal("new change retained redo")
	}
	if history.UndoDepth() != 2 {
		t.Fatalf("new change depth = %d", history.UndoDepth())
	}
}

func TestHistoryBoundsTheUndoStackAtTwoHundredFrames(t *testing.T) {
	history := NewHistory(&fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	document, root := stateDocument(t, "abc")
	before := stateFrame(t, document, root, 0)
	for index := 0; index < HistoryLimit+5; index++ {
		history.Record(before, stateVisual(0, model.UTF16Offset(index)), true, false)
	}
	if history.UndoDepth() != HistoryLimit {
		t.Fatalf("undo depth = %d, want %d", history.UndoDepth(), HistoryLimit)
	}
}

func TestHistoryRestorePreservesSemanticMarksAndMonotonicIdentity(t *testing.T) {
	document, root := stateDocument(t, "abc")
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		_, err := draft.AddMark(root, model.NewUTF16Range(0, 1), 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := document.Snapshot()
	node, _ := snapshot.Node(root)
	var first model.MarkID
	for _, mark := range node.Marks() {
		first = mark.ID()
	}
	active := first
	frame, ok := NewFrame(
		node,
		model.CollapsedSelection(model.NewTextAnchor(root, 1, model.AffinityRight, snapshot.Revision())),
		stateVisual(snapshot.Revision(), 1),
		&active,
	)
	if !ok {
		t.Fatal("frame rejected")
	}
	history := NewHistory(&fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	history.Record(frame, frame.Visual(), true, false)

	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetMarkLevel(root, first, 0); err != nil {
			return err
		}
		_, err := draft.AddMark(root, model.NewUTF16Range(1, 2), 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	currentSnapshot := document.Snapshot()
	currentNode, _ := currentSnapshot.Node(root)
	current, ok := NewFrame(currentNode, frame.Selection(), frame.Visual(), nil)
	if !ok {
		t.Fatal("current frame rejected")
	}
	ok, err := history.Undo(current, func(source Frame) error {
		_, err := document.Transact(func(draft *model.DocumentDraft) error {
			node := source.Node()
			return draft.RestoreContent(&node)
		})
		return err
	})
	if err != nil || !ok {
		t.Fatalf("undo = %v %v", ok, err)
	}
	restoredNode, _ := document.Snapshot().Node(root)
	if restoredNode.MarkCount() != 1 || restoredNode.Marks()[0].ID() != first {
		t.Fatalf("restored marks = %#v", restoredNode.Marks())
	}
	var next model.MarkID
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		var err error
		next, err = draft.AddMark(root, model.NewUTF16Range(1, 2), 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if next <= first || next.Value() < 3 {
		t.Fatalf("mark identity reused: first=%s next=%s", first, next)
	}
}

package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func mustDocument(t *testing.T, kind string) *Document {
	t.Helper()
	document, err := NewDocument(NewNodeTypeID(kind))
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	return document
}

func mustTransact(t *testing.T, document *Document, operation func(*DocumentDraft) error) TransactionOutcome {
	t.Helper()
	outcome, err := document.Transact(operation)
	if err != nil {
		t.Fatalf("Transact: %v", err)
	}
	return outcome
}

func TestSemanticActionsBuildStableTree(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	var paragraph NodeID
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		paragraph, err = draft.CreateNode(root, 0, NewNodeTypeID("paragraph"))
		if err != nil {
			return err
		}
		if err := draft.SetTextValue(paragraph, "semantic text"); err != nil {
			return err
		}
		return draft.SetAttribute(paragraph, "level", IntegerValue(2))
	})
	if outcome.Revision != 1 {
		t.Fatalf("revision = %d, want 1", outcome.Revision)
	}
	wantChanges := []Change{
		{Kind: ChangeUpdated, Node: root},
		{Kind: ChangeCreated, Node: paragraph},
	}
	if !slices.Equal(outcome.Changes, wantChanges) {
		t.Fatalf("changes = %#v, want %#v", outcome.Changes, wantChanges)
	}
	node, ok := document.Snapshot().Node(paragraph)
	if !ok {
		t.Fatal("missing paragraph")
	}
	if parent, ok := node.Parent(); !ok || parent != root {
		t.Fatalf("parent = %d,%v, want %d,true", parent, ok, root)
	}
	if got := node.Text().String(); got != "semantic text" {
		t.Fatalf("text = %q", got)
	}
	if got, ok := node.Attribute("level"); !ok || got != IntegerValue(2) {
		t.Fatalf("attribute = %#v,%v", got, ok)
	}
}

func TestSemanticActionApply(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	var created NodeID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		id, ok, err := draft.Apply(SemanticAction{
			Kind:     ActionCreateNode,
			Parent:   root,
			NodeType: "paragraph",
		})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("create action did not report a node")
		}
		created = id
		text := "hello"
		if _, ok, err := draft.Apply(SemanticAction{
			Kind: ActionSetText,
			Node: created,
			Text: &text,
		}); err != nil || ok {
			t.Fatalf("set text = %v,%v", ok, err)
		}
		if _, _, err := draft.Apply(SemanticAction{
			Kind:  ActionSetAttribute,
			Node:  created,
			Key:   "done",
			Value: BoolValue(true),
		}); err != nil {
			return err
		}
		if _, _, err := draft.Apply(SemanticAction{
			Kind: ActionRemoveAttribute,
			Node: created,
			Key:  "done",
		}); err != nil {
			return err
		}
		return nil
	})
	if created == 0 {
		t.Fatal("missing created node")
	}
}

func TestFailedTransactionDoesNotPublishPartialState(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	_, err := document.Transact(func(draft *DocumentDraft) error {
		if _, err := draft.CreateNode(root, 0, "paragraph"); err != nil {
			return err
		}
		return draft.SetAttribute(root, "", NullValue())
	})
	var documentError *DocumentError
	if !errors.As(err, &documentError) || documentError.Kind != DocumentErrorEmptyAttributeKey {
		t.Fatalf("error = %v, want empty attribute key", err)
	}
	if document.Revision() != 0 || len(document.Snapshot().Nodes()) != 1 {
		t.Fatalf("failed transaction published state")
	}
}

func TestAppendAndReplaceTextAcrossUnicode(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(root, "a🦀")
	})
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.AppendText(root, "中")
	})
	node, _ := document.Snapshot().Node(root)
	if got := node.Text().String(); got != "a🦀中" {
		t.Fatalf("text = %q", got)
	}
	if got := node.Text().UTF16Len(); got != 4 {
		t.Fatalf("UTF-16 length = %d", got)
	}
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextUTF16(root, 1, 3, "🐻")
	})
	node, _ = document.Snapshot().Node(root)
	if got := node.Text().String(); got != "a🐻中" {
		t.Fatalf("replacement text = %q", got)
	}
}

func TestEqualUpdatesDoNotAdvanceRevision(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetAttribute(root, "mode", TextValue("plain"))
	})
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetAttribute(root, "mode", TextValue("plain"))
	})
	if outcome.Revision != 1 || len(outcome.Changes) != 0 {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestNoOpTextAndAttributeWritesStillMarkDraftWritten(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetText(root, nil); err != nil {
			return err
		}
		if !draft.WasWritten() {
			t.Fatal("equal absent text did not mark draft written")
		}
		return draft.RemoveAttribute(root, "missing")
	})
	if len(outcome.Changes) != 0 || outcome.Revision != 0 {
		t.Fatalf("no-op outcome = %#v", outcome)
	}
}

func TestRemovingNodeRemovesSubtree(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	var parent, child NodeID
	mustTransact(t, document, func(draft *DocumentDraft) error {
		var err error
		parent, err = draft.CreateNode(root, 0, "section")
		if err != nil {
			return err
		}
		child, err = draft.CreateNode(parent, 0, "paragraph")
		return err
	})
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.RemoveNode(parent)
	})
	if !slices.Contains(outcome.Changes, Change{Kind: ChangeRemoved, Node: parent}) ||
		!slices.Contains(outcome.Changes, Change{Kind: ChangeRemoved, Node: child}) {
		t.Fatalf("changes = %#v", outcome.Changes)
	}
	if _, ok := document.Snapshot().Node(parent); ok {
		t.Fatal("parent survived")
	}
	if _, ok := document.Snapshot().Node(child); ok {
		t.Fatal("child survived")
	}
}

func TestUTF16ReplacementMatchesIndependentStringModel(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"", "plain", "a🦀中🐻z", "e\u0301👩‍💻\r\n"} {
		t.Run(source, func(t *testing.T) {
			base := mustDocument(t, "document")
			root := base.Snapshot().Root()
			mustTransact(t, base, func(draft *DocumentDraft) error {
				return draft.SetTextValue(root, source)
			})
			before := base.Snapshot()
			type boundary struct {
				unit int
				byte int
			}
			boundaries := []boundary{{0, 0}}
			units := 0
			for byteIndex, r := range source {
				units += utf16.RuneLen(r)
				boundaries = append(boundaries, boundary{units, byteIndex + len(string(r))})
			}
			find := func(offset int) (boundary, bool) {
				for _, candidate := range boundaries {
					if candidate.unit == offset {
						return candidate, true
					}
				}
				return boundary{}, false
			}
			for from := 0; from <= units+1; from++ {
				for to := 0; to <= units+1; to++ {
					candidate := base.Clone()
					_, err := candidate.Transact(func(draft *DocumentDraft) error {
						return draft.ReplaceTextUTF16(
							root,
							UTF16Offset(from),
							UTF16Offset(to),
							"界🐻",
						)
					})
					start, startOK := find(from)
					end, endOK := find(to)
					if startOK && endOK && from <= to {
						if err != nil {
							t.Fatalf("%d..%d rejected: %v", from, to, err)
						}
						expected := source[:start.byte] + "界🐻" + source[end.byte:]
						node, _ := candidate.Snapshot().Node(root)
						if got := node.Text().String(); got != expected {
							t.Fatalf("%d..%d got %q, want %q", from, to, got, expected)
						}
					} else {
						if err == nil {
							t.Fatalf("%d..%d accepted", from, to)
						}
						if candidate.Revision() != before.Revision() {
							t.Fatalf("failed transaction published revision")
						}
					}
				}
			}
		})
	}
}

func TestUTF16ValidationAcrossRopeChunks(t *testing.T) {
	t.Parallel()
	source := strings.Repeat("a🦀中e\u0301", 2048)
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(root, source)
	})
	node, _ := document.Snapshot().Node(root)
	offset := UTF16Offset(0)
	for _, r := range source {
		if err := node.ValidateTextRangeUTF16(offset, offset); err != nil {
			t.Fatalf("boundary %d: %v", offset, err)
		}
		if utf16.RuneLen(r) == 2 {
			err := node.ValidateTextRangeUTF16(offset+1, offset+1)
			var documentError *DocumentError
			if !errors.As(err, &documentError) ||
				documentError.Kind != DocumentErrorInvalidTextBoundary {
				t.Fatalf("surrogate interior %d: %v", offset+1, err)
			}
		}
		offset += UTF16Offset(utf16.RuneLen(r))
	}
	if err := node.ValidateTextRangeUTF16(offset, offset); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidRangeAndEmptyReplacementDoNotAllocateText(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.ReplaceTextUTF16(root, 0, 1, "oops"); err == nil {
			t.Fatal("invalid range accepted")
		}
		if draft.WasWritten() {
			t.Fatal("failed range recorded a write")
		}
		if node, ok := draft.Node(root); !ok || node.TextPresent() {
			t.Fatal("failed range allocated text")
		}
		return draft.SetAttribute(root, "edited", BoolValue(true))
	})
	before := document.Snapshot()
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.ReplaceTextUTF16(root, 0, 0, ""); err != nil {
			return err
		}
		if !draft.WasWritten() {
			t.Fatal("empty replacement did not mark written")
		}
		return nil
	})
	if len(outcome.Changes) != 0 || document.Snapshot() != before {
		t.Fatal("empty absent-to-absent replacement published")
	}
}

func TestTextEditFootprintsFollowSuccessfulOperationsAndCheckpoints(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(root, "ab"); err != nil {
			return err
		}
		checkpoint := draft.Checkpoint()
		if err := draft.AppendText(root, "🦀"); err != nil {
			return err
		}
		if count := len(draft.TextEdits()); count != 2 {
			t.Fatalf("text edits = %d, want 2", count)
		}
		draft.Rollback(checkpoint)
		edits := draft.TextEdits()
		if len(edits) != 1 {
			t.Fatalf("rolled back text edits = %d, want 1", len(edits))
		}
		if edits[0].RangeUTF16() != NewUTF16Range(0, 0) ||
			edits[0].InsertedUTF16Len() != 2 {
			t.Fatalf("first edit = %#v", edits[0])
		}
		return nil
	})
}

func TestCheckpointRestoresTreeIdentitySupplyAndWrittenFlag(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		checkpoint := draft.Checkpoint()
		first, err := draft.CreateNode(root, 0, "discarded")
		if err != nil {
			return err
		}
		draft.Rollback(checkpoint)
		if draft.WasWritten() {
			t.Fatal("rollback retained written flag")
		}
		second, err := draft.CreateNode(root, 0, "paragraph")
		if err != nil {
			return err
		}
		if first != second {
			t.Fatalf("rollback did not restore identity supply: %d != %d", first, second)
		}
		return nil
	})
}

func TestSnapshotsRemainImmutableAndTextRootIdentityIsRetainedForNoOps(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.SetTextValue(root, "a🦀")
	})
	original := document.Snapshot()
	originalNode, _ := original.Node(root)
	retained, _ := originalNode.TextRoot()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetAttribute(root, "emphasis", IntegerValue(2)); err != nil {
			return err
		}
		if err := draft.SetTextValue(root, "a🦀"); err != nil {
			return err
		}
		return draft.ReplaceTextUTF16(root, 1, 3, "🦀")
	})
	updatedNode, _ := document.Snapshot().Node(root)
	updated, _ := updatedNode.TextRoot()
	if !retained.Same(updated) {
		t.Fatal("equal no-op text edit replaced text root identity")
	}
	originalNode, _ = original.Node(root)
	if got := originalNode.Text().String(); got != "a🦀" {
		t.Fatalf("old snapshot changed: %q", got)
	}

	draft := document.BeginDraft()
	checkpoint := draft.Checkpoint()
	if err := draft.AppendText(root, "中"); err != nil {
		t.Fatal(err)
	}
	changedNode, _ := draft.Node(root)
	changed, _ := changedNode.TextRoot()
	if retained.Same(changed) {
		t.Fatal("text edit retained old root identity")
	}
	draft.Rollback(checkpoint)
	restoredNode, _ := draft.Node(root)
	restored, _ := restoredNode.TextRoot()
	if !retained.Same(restored) {
		t.Fatal("rollback did not restore retained root identity")
	}

	mustTransact(t, document, func(draft *DocumentDraft) error {
		return draft.ReplaceTextUTF16(root, 1, 3, "🐻")
	})
	finalNode, _ := document.Snapshot().Node(root)
	final, _ := finalNode.TextRoot()
	if retained.Same(final) {
		t.Fatal("text edit retained old root identity")
	}
	if retained.String() != "a🦀" || final.String() != "a🐻" {
		t.Fatalf("roots = %q, %q", retained.String(), final.String())
	}
}

func TestFinalChangesCancelAndDeduplicate(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	outcome := mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetAttribute(root, "mode", TextValue("temporary")); err != nil {
			return err
		}
		return draft.RemoveAttribute(root, "mode")
	})
	if len(outcome.Changes) != 0 || outcome.Revision != 0 {
		t.Fatalf("cancelled changes = %#v", outcome)
	}
	outcome = mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetAttribute(root, "a", BoolValue(true)); err != nil {
			return err
		}
		return draft.SetAttribute(root, "b", BoolValue(true))
	})
	if len(outcome.Changes) != 1 || outcome.Changes[0] != (Change{Kind: ChangeUpdated, Node: root}) {
		t.Fatalf("deduped changes = %#v", outcome.Changes)
	}
}

func TestIdentityAndRevisionExhaustionAreAtomic(t *testing.T) {
	t.Parallel()
	t.Run("identity", func(t *testing.T) {
		document := mustDocument(t, "document")
		root := document.Snapshot().Root()
		draft := document.BeginDraft()
		draft.state.nextID = ^uint64(0)
		before := draft.state
		_, err := draft.CreateNode(root, 0, "paragraph")
		var documentError *DocumentError
		if !errors.As(err, &documentError) ||
			documentError.Kind != DocumentErrorIdentityExhausted {
			t.Fatalf("error = %v", err)
		}
		if draft.state != before || len(draft.FinalChanges()) != 0 {
			t.Fatal("identity exhaustion mutated draft")
		}
	})
	t.Run("revision", func(t *testing.T) {
		document := mustDocument(t, "document")
		document.revision = ^uint64(0)
		root := document.Snapshot().Root()
		before := document.Snapshot()
		_, err := document.Transact(func(draft *DocumentDraft) error {
			return draft.SetAttribute(root, "x", NullValue())
		})
		var documentError *DocumentError
		if !errors.As(err, &documentError) ||
			documentError.Kind != DocumentErrorRevisionExhausted {
			t.Fatalf("error = %v", err)
		}
		if document.Snapshot() != before {
			t.Fatal("revision exhaustion published")
		}
	})
}

func TestAttributeValuesAndNodeIdentity(t *testing.T) {
	t.Parallel()
	values := []AttributeValue{
		NullValue(),
		BoolValue(true),
		IntegerValue(-42),
		TextValue("text"),
	}
	want := []string{"null", "true", "-42", "text"}
	for index, value := range values {
		if got := value.String(); got != want[index] {
			t.Fatalf("value %d = %q, want %q", index, got, want[index])
		}
	}
	if _, ok := BoolValue(true).Text(); ok {
		t.Fatal("boolean reported text payload")
	}
	for index, value := range values {
		if got := value.Kind(); got != AttributeKind(index) {
			t.Fatalf("value %d kind = %d", index, got)
		}
	}
	if got := NodeID(7).String(); got != "node-7" {
		t.Fatalf("node identity = %q", got)
	}
	if got := MarkID(3).String(); got != "mark-3" {
		t.Fatalf("mark identity = %q", got)
	}
}

func TestDocumentTextMustBeValidUTF8(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	root := document.Snapshot().Root()
	invalid := string([]byte{0xff, 'a'})
	_, err := document.Transact(func(draft *DocumentDraft) error {
		return draft.SetTextValue(root, invalid)
	})
	var documentError *DocumentError
	if !errors.As(err, &documentError) ||
		documentError.Kind != DocumentErrorInvalidDocument {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
	if document.Revision() != 0 {
		t.Fatal("invalid UTF-8 was published")
	}
}

func TestValidateRejectsBrokenStructureAndRootMutation(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "document")
	root := document.Snapshot().Root()
	_, err := document.Transact(func(draft *DocumentDraft) error {
		return draft.RemoveNode(root)
	})
	var documentError *DocumentError
	if !errors.As(err, &documentError) || documentError.Kind != DocumentErrorRootMutation {
		t.Fatalf("root removal = %v", err)
	}
	draft := document.BeginDraft()
	child, err := draft.CreateNode(root, 0, "paragraph")
	if err != nil {
		t.Fatal(err)
	}
	draft.state.nodes[child].parent = root + 1
	if err := draft.Validate(); err == nil {
		t.Fatal("broken parent link validated")
	}
}

func TestSetTextClearsMarksAndRestoreContentKeepsIdentitySupply(t *testing.T) {
	t.Parallel()
	document := mustDocument(t, "text")
	root := document.Snapshot().Root()
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(root, "abc"); err != nil {
			return err
		}
		_, err := draft.AddMark(root, NewUTF16Range(0, 3), 1)
		return err
	})
	source, _ := document.Snapshot().Node(root)
	sourceState, _ := document.Snapshot().state.nodes[root], true
	mustTransact(t, document, func(draft *DocumentDraft) error {
		if err := draft.SetTextValue(root, "replacement"); err != nil {
			return err
		}
		node, _ := draft.Node(root)
		if node.MarkCount() != 0 {
			t.Fatal("SetText retained marks")
		}
		return draft.RestoreContent(source)
	})
	restored, _ := document.Snapshot().Node(root)
	if restored.Text().String() != "abc" || restored.MarkCount() != 1 {
		t.Fatalf("restored node text=%q marks=%d", restored.Text().String(), restored.MarkCount())
	}
	draft := document.BeginDraft()
	id, err := draft.AddMark(root, NewUTF16Range(0, 1), 1)
	if err != nil {
		t.Fatal(err)
	}
	if id.Value() <= sourceState.nextMarkID-1 {
		t.Fatalf("mark id %d reused identity below %d", id, sourceState.nextMarkID)
	}
}

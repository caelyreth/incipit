package kernel

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func ruleContextFixture(
	t *testing.T,
	access DocumentAccess,
	event Event,
) (*RuleContext, *EngineDraft) {
	t.Helper()
	engine := engineFixture(t)
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	rule := &LinkedRule{
		Key:     RuleKey{Owner: "test", ID: "authority"},
		Effects: NewEffectSet(nil, nil).WithDocument(access),
	}
	return NewRuleContext(draft, rule, event, nil), draft
}

func requireEngineErrorKind(t *testing.T, err error, want EngineErrorKind) {
	t.Helper()
	var engineErr *EngineError
	if !errors.As(err, &engineErr) {
		t.Fatalf("error = %T %v, want *EngineError", err, err)
	}
	if engineErr.Kind != want {
		t.Fatalf("kind = %d, want %d (%v)", engineErr.Kind, want, err)
	}
}

func TestRuleContextChangedRequiresDocumentReadAuthority(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		context, _ := ruleContextFixture(t, DocumentAccessNone, NewCommandEvent("test"))
		_, err := context.Changed(DocumentStateKey())
		requireEngineErrorKind(t, err, EngineErrorUndeclaredDocumentRead)
	})

	t.Run("read", func(t *testing.T) {
		context, _ := ruleContextFixture(t, DocumentAccessRead, NewCommandEvent("test"))
		changed, err := context.Changed(DocumentStateKey())
		if err != nil || changed {
			t.Fatalf("changed = %v, err = %v", changed, err)
		}
	})

	t.Run("write", func(t *testing.T) {
		context, _ := ruleContextFixture(t, DocumentAccessWrite, NewCommandEvent("test"))
		document, err := context.DocumentMut()
		if err != nil {
			t.Fatal(err)
		}
		root := document.Root()
		if err := document.AppendText(root, "!"); err != nil {
			t.Fatal(err)
		}
		changed, err := context.Changed(DocumentStateKey())
		if err != nil || !changed {
			t.Fatalf("changed = %v, err = %v", changed, err)
		}
	})
}

func TestRuleContextDocumentAccessorsReturnSpecificAuthorityErrors(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		context, _ := ruleContextFixture(t, DocumentAccessNone, NewCommandEvent("test"))
		_, err := context.Document()
		requireEngineErrorKind(t, err, EngineErrorUndeclaredDocumentRead)
		if got := err.Error(); got != "undeclared read of the semantic document" {
			t.Fatalf("message = %q", got)
		}
	})

	context, _ := ruleContextFixture(t, DocumentAccessRead, NewCommandEvent("test"))
	if _, err := context.Document(); err != nil {
		t.Fatalf("read access rejected: %v", err)
	}
	_, err := context.DocumentMut()
	requireEngineErrorKind(t, err, EngineErrorUndeclaredDocumentWrite)
	if got := err.Error(); got != "undeclared write of the semantic document" {
		t.Fatalf("message = %q", got)
	}
}

func TestRuleContextTextInputRequiresDocumentWriteAuthority(t *testing.T) {
	t.Parallel()
	for _, access := range []DocumentAccess{DocumentAccessNone, DocumentAccessRead} {
		engine := engineFixture(t)
		root := engine.Snapshot().Document().Root()
		event := NewTextEvent(
			NewTextInput(
				NewTextTarget(root, model.EmptyUTF16Range(0), engine.Revision()),
				"a",
			),
			EventOriginTyping,
		)
		context, _ := ruleContextFixture(t, access, event)
		_, err := context.ApplyTextInput()
		requireEngineErrorKind(t, err, EngineErrorUndeclaredDocumentWrite)

		context, _ = ruleContextFixture(t, access, event)
		_, _, _, err = context.ReplaceTextInputMarked(nil)
		requireEngineErrorKind(t, err, EngineErrorUndeclaredDocumentWrite)
	}
}

func TestReplaceTextInputMarkedUsesNormalizedContentForHeadAndMarks(t *testing.T) {
	t.Parallel()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	root := document.Snapshot().Root()
	var markID model.MarkID
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(root, "ab"); err != nil {
			return err
		}
		var err error
		markID, err = draft.AddMark(root, model.EmptyUTF16Range(1), "strong", 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(document, NewResourceStore(nil))
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	event := NewTextEvent(
		NewTextInput(
			NewTextTarget(root, model.EmptyUTF16Range(1), engine.Revision()),
			"🦀\r\nx\r",
		),
		EventOriginTyping,
	)
	rule := &LinkedRule{
		Key:     RuleKey{Owner: "test", ID: "normalized-input"},
		Effects: NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
	}
	context := NewRuleContext(draft, rule, event, nil)
	node, head, handled, err := context.ReplaceTextInputMarked(&markID)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || node != root {
		t.Fatalf("node = %d, handled = %v", node, handled)
	}
	updated, ok := draft.Document().Node(root)
	if !ok {
		t.Fatal("updated node is missing")
	}
	const normalized = "🦀\nx\n"
	if got := updated.Text().String(); got != "a"+normalized+"b" {
		t.Fatalf("text = %q", got)
	}
	wantHead := 1 + model.StringLenUTF16(normalized)
	if head != wantHead {
		t.Fatalf("head = %d, want %d", head, wantHead)
	}
	edits := draft.Document().TextEdits()
	if len(edits) != 1 || int(edits[0].InsertedUTF16Len()) != model.StringLenUTF16(normalized) {
		t.Fatalf("text edits = %#v", edits)
	}
	marks := updated.Marks()
	if len(marks) != 1 || marks[0].Range() != model.NewUTF16Range(1, model.UTF16Offset(wantHead)) {
		t.Fatalf("marks = %#v", marks)
	}
}

func TestEngineErrorStringIsNilSafe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  *EngineError
		want string
	}{
		{nil, "<nil>"},
		{&EngineError{Kind: EngineErrorDocument}, "semantic document operation failed"},
		{&EngineError{Kind: EngineErrorResource}, "resource operation failed"},
		{&EngineError{Kind: EngineErrorInput}, "input operation failed"},
		{&EngineError{Kind: EngineErrorUndeclaredDocumentRead}, "undeclared read of the semantic document"},
		{&EngineError{Kind: EngineErrorUndeclaredDocumentWrite}, "undeclared write of the semantic document"},
	}
	for _, test := range cases {
		if got := test.err.Error(); got != test.want {
			t.Fatalf("(*EngineError)(%#v).Error() = %q, want %q", test.err, got, test.want)
		}
	}
	var err *EngineError
	if unwrapped := err.Unwrap(); unwrapped != nil {
		t.Fatalf("nil unwrap = %v", unwrapped)
	}
}

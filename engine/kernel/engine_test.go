package kernel

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

func engineFixture(t *testing.T) *Engine {
	t.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		t.Fatal(err)
	}
	root := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(root, "abc")
	}); err != nil {
		t.Fatal(err)
	}
	resources := NewResourceStore([]ResourceDescriptor{{
		ID:       "enabled",
		Lifetime: ResourceDocumentLifetime,
		Kind:     ResourceBool,
		Initial:  BoolResource(false),
	}})
	return NewEngine(document, resources)
}

func TestTransactionPublishesBothDomainsAtOneRevision(t *testing.T) {
	t.Parallel()
	engine := engineFixture(t)
	root := engine.Snapshot().Document().Root()
	effects := NewEffectSet([]ResourceID{"enabled"}, []ResourceID{"enabled"})
	outcome, err := Transact(engine, func(draft *EngineDraft) (string, error) {
		if err := draft.Document().ReplaceTextUTF16(root, model.UTF16Offset(1), model.UTF16Offset(2), "界"); err != nil {
			return "", err
		}
		if err := draft.Resources().Write(effects).Set("enabled", BoolResource(true)); err != nil {
			return "", err
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Revision != engine.Revision() || outcome.Revision != 2 {
		t.Fatalf("revision = %d engine=%d", outcome.Revision, engine.Revision())
	}
	if len(outcome.Changes.Document()) != 1 || len(outcome.Changes.Resources()) != 1 {
		t.Fatalf("changes = %#v", outcome.Changes)
	}
	node, _ := engine.Snapshot().Document().Node(root)
	if got := node.Text().String(); got != "a界c" {
		t.Fatalf("text = %q", got)
	}
}

func TestPreparationFailurePublishesNothing(t *testing.T) {
	t.Parallel()
	engine := engineFixture(t)
	before := engine.Snapshot()
	root := before.Document().Root()
	_, err := TransactPrepared(engine, func(draft *EngineDraft) (struct{}, error) {
		return struct{}{}, draft.Document().AppendText(root, "!")
	}, func(EngineView, EngineChanges, struct{}) (struct{}, error) {
		return struct{}{}, errors.New("preparation failed")
	})
	if err == nil {
		t.Fatal("preparation failure accepted")
	}
	after := engine.Snapshot()
	if after.Revision() != before.Revision() {
		t.Fatal("failed preparation advanced revision")
	}
	node, _ := after.Document().Node(root)
	if node.Text().String() != "abc" {
		t.Fatal("failed preparation published text")
	}
}

func TestNoopTransactionRetainsRevisionAndIdentity(t *testing.T) {
	t.Parallel()
	engine := engineFixture(t)
	before := engine.Snapshot()
	root := before.Document().Root()
	outcome, err := Transact(engine, func(draft *EngineDraft) (struct{}, error) {
		if err := draft.Document().SetTextValue(root, "abc"); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Revision != before.Revision() || len(outcome.Changes.Document()) != 0 {
		t.Fatalf("noop outcome = %#v", outcome)
	}
}

func TestCheckpointRestoresDocumentResourcesAndFlags(t *testing.T) {
	t.Parallel()
	engine := engineFixture(t)
	root := engine.Snapshot().Document().Root()
	effects := NewEffectSet(nil, []ResourceID{"enabled"})
	_, err := Transact(engine, func(draft *EngineDraft) (struct{}, error) {
		checkpoint := draft.Checkpoint()
		if err := draft.Document().AppendText(root, "x"); err != nil {
			return struct{}{}, err
		}
		if err := draft.Resources().Write(effects).Set("enabled", BoolResource(true)); err != nil {
			return struct{}{}, err
		}
		draft.Rollback(checkpoint)
		if draft.Document().WasWritten() || draft.Resources().WasWritten("enabled") {
			return struct{}{}, errors.New("rollback retained writes")
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if engine.Revision() != 1 {
		t.Fatalf("revision = %d", engine.Revision())
	}
}

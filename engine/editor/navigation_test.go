package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

type editingFixture struct {
	document *model.Document
	node     model.NodeID
}

func newEditingFixture(t *testing.T, text string, marks ...model.UTF16Range) editingFixture {
	t.Helper()
	document, node := emptyBlockDocument(t)
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, text); err != nil {
			return err
		}
		for _, mark := range marks {
			if _, err := draft.AddMark(node, mark, EmphasisMark, 1); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return editingFixture{document: document, node: node}
}

func (f editingFixture) nodeValue(t *testing.T) *model.Node {
	t.Helper()
	node, ok := f.document.Snapshot().Node(f.node)
	if !ok {
		t.Fatal("fixture node missing")
	}
	return node
}

func (f editingFixture) selection(t *testing.T, anchor, head model.UTF16Offset) model.TextSelection {
	t.Helper()
	revision := f.document.Snapshot().Revision()
	return model.NewTextSelection(
		model.NewTextAnchor(f.node, anchor, model.AffinityRight, revision),
		model.NewTextAnchor(f.node, head, model.AffinityRight, revision),
	)
}

func TestNavigationReturnRequiresEightGraphemesAndExpires(t *testing.T) {
	t.Parallel()
	fixture := newEditingFixture(t, "a\u0301👩‍👩‍👧‍👦🇨🇳z12345678")
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	start := fixture.selection(t, 0, 0)
	var nav Navigation
	nav.Remember(start)
	pointer := fixture.selection(t, 26, 26)
	if err := nav.SelectPointer(fixture.nodeValue(t).Text().String(), pointer, now); err != nil {
		t.Fatal(err)
	}
	target, ok := nav.Returning()
	if !ok || target.ExpiresAt != now.Add(14*time.Second) {
		t.Fatalf("returning=%#v ok=%v", target, ok)
	}
	near := fixture.selection(t, 2, 2)
	nav.Remember(fixture.selection(t, 0, 0))
	if err := nav.SelectPointer(fixture.nodeValue(t).Text().String(), near, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := nav.Returning(); ok {
		t.Fatal("short pointer move offered return")
	}
}

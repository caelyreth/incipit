package editor

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestSessionTextMovementUsesGraphemesAndSelectionEdges(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "a\u0301🦀b", false)
	begin := model.NewTextSelection(
		model.NewTextAnchor(node, 0, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 0, model.AffinityRight, session.Runtime().Revision()),
	)
	if err := session.Select(begin); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.ExecuteText(TextRight); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("right outcome=%v err=%v", outcome, err)
	}
	if head := session.Selection().Head().OffsetUTF16(); head != 2 {
		t.Fatalf("grapheme head = %d", head)
	}
	if _, err := session.ExecuteText(TextRight); err != nil {
		t.Fatal(err)
	}
	if head := session.Selection().Head().OffsetUTF16(); head != 4 {
		t.Fatalf("supplementary head = %d", head)
	}
	if _, err := session.ExecuteText(TextNewline); err != nil {
		t.Fatal(err)
	}
}

func TestSessionHistoryRestoresSemanticAndVisualState(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abc", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 3, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReplaceSelection("d", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.ExecuteHistory(HistoryUndo); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("undo outcome=%v err=%v", outcome, err)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "abc" {
		t.Fatalf("undo text = %q", nodeValue.Text().String())
	}
	if outcome, err := session.ExecuteHistory(HistoryRedo); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("redo outcome=%v err=%v", outcome, err)
	}
	nodeValue, _ = session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "abcd" {
		t.Fatalf("redo text = %q", nodeValue.Text().String())
	}
}

func TestSessionCommandsRejectActiveComposition(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	if err := session.BeginComposition(); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ExecuteText(TextRight); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("text command during composition = %v", err)
	}
	if _, err := session.ExecuteHistory(HistoryUndo); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("history command during composition = %v", err)
	}
}

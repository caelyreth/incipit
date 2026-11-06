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
	if outcome, err := session.press(textRight); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("right outcome=%v err=%v", outcome, err)
	}
	if head := session.Selection().Head().OffsetUTF16(); head != 2 {
		t.Fatalf("grapheme head = %d", head)
	}
	if _, err := session.press(textRight); err != nil {
		t.Fatal(err)
	}
	if head := session.Selection().Head().OffsetUTF16(); head != 4 {
		t.Fatalf("supplementary head = %d", head)
	}
	if _, err := session.press(textNewline); err != nil {
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
	if _, err := session.replaceSelection("d", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.doHistory(historyUndo); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("undo outcome=%v err=%v", outcome, err)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "abc" {
		t.Fatalf("undo text = %q", nodeValue.Text().String())
	}
	if outcome, err := session.doHistory(historyRedo); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("redo outcome=%v err=%v", outcome, err)
	}
	nodeValue, _ = session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "abcd" {
		t.Fatalf("redo text = %q", nodeValue.Text().String())
	}
}

func TestSessionHistoryRestoresMarkerOnlyVisualSelection(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	typeSession(t, session, "*")
	typeSession(t, session, "abc")
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.doFormat(markupStrong); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("strong outcome=%v err=%v", outcome, err)
	}
	if session.Projection().Text() != "**abc**" {
		t.Fatalf("strong projection = %q", session.Projection().Text())
	}

	// Selection changes do not create history entries. Capture the marker-only
	// selection in the redo frame as well as the undo frame.
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.doHistory(historyUndo); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("undo outcome=%v err=%v", outcome, err)
	}
	if session.Projection().Text() != "*abc*" {
		t.Fatalf("undo projection = %q", session.Projection().Text())
	}
	if visual, err := session.VisualSelection(); err != nil ||
		visual != model.NewUTF16Range(0, 1) {
		t.Fatalf("undo visual selection=%#v err=%v", visual, err)
	}

	if outcome, err := session.doHistory(historyRedo); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("redo outcome=%v err=%v", outcome, err)
	}
	if session.Projection().Text() != "**abc**" {
		t.Fatalf("redo projection = %q", session.Projection().Text())
	}
	if visual, err := session.VisualSelection(); err != nil ||
		visual != model.NewUTF16Range(0, 1) {
		t.Fatalf("redo visual selection=%#v err=%v", visual, err)
	}
}

func TestSessionCommandsRejectActiveComposition(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	if err := session.BeginComposition(); err != nil {
		t.Fatal(err)
	}
	if _, err := session.press(textRight); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("text command during composition = %v", err)
	}
	if _, err := session.doHistory(historyUndo); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("history command during composition = %v", err)
	}
}

package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestToolbarStateTracksFormattingAndHistory(t *testing.T) {
	session, node := newSessionFixture(t, "alpha", false)
	state := session.ToolbarState()
	if !state.EmphasisEnabled || state.Emphasis || state.CanUndo || state.CanRedo {
		t.Fatalf("initial toolbar = %#v", state)
	}
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 0, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 5, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if outcome, err := session.ExecuteMarkup(MarkupEmphasis); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("markup = %v,%v", outcome, err)
	}
	state = session.ToolbarState()
	if !state.Emphasis || !state.CanUndo || state.Strong {
		t.Fatalf("selected toolbar = %#v", state)
	}
	if outcome, err := session.ExecuteHistory(HistoryUndo); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("undo = %v,%v", outcome, err)
	}
	state = session.ToolbarState()
	if state.Emphasis || !state.CanRedo {
		t.Fatalf("undo toolbar = %#v", state)
	}
}

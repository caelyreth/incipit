package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestSessionExecutesMarkupCommandsAndTogglesMarks(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcd", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	outcome, err := session.ExecuteMarkup(MarkupEmphasis)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != kernel.OutcomeHandled || session.Projection().Text() != "a*bc*d" {
		t.Fatalf("format outcome=%v projection=%q", outcome, session.Projection().Text())
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if len(nodeValue.Marks()) != 1 || nodeValue.Marks()[0].Level() != 1 {
		t.Fatalf("marks = %#v", nodeValue.Marks())
	}
	if _, err := session.ExecuteMarkup(MarkupEmphasis); err != nil {
		t.Fatal(err)
	}
	nodeValue, _ = session.Runtime().Snapshot().Document().Node(node)
	if len(nodeValue.Marks()) != 0 || session.Projection().Text() != "abcd" {
		t.Fatalf("toggle-off marks=%#v projection=%q", nodeValue.Marks(), session.Projection().Text())
	}
}

func TestSessionExecutesWordMovementSelectionAndDeletion(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "one two three", false)
	if outcome, err := session.ExecuteWord(WordNext); err != nil ||
		outcome != kernel.OutcomeHandled ||
		session.Selection().Head().OffsetUTF16() != 3 {
		t.Fatalf("next outcome=%v head=%d err=%v", outcome, session.Selection().Head().OffsetUTF16(), err)
	}
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 5, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 5, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ExecuteWord(SelectWord); err != nil {
		t.Fatal(err)
	}
	range16, err := session.Selection().Range()
	if err != nil {
		t.Fatal(err)
	}
	if range16 != model.NewUTF16Range(4, 7) {
		t.Fatalf("selected word = %#v", range16)
	}
	if _, err := session.ExecuteWord(WordDeletePrevious); err != nil {
		t.Fatal(err)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "one  three" {
		t.Fatalf("delete previous word text = %q", nodeValue.Text().String())
	}
}

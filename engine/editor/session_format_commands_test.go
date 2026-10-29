package editor

import (
	"errors"
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

func TestSessionStrongMarkupHonorsAllowStrong(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abc", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 0, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetSetting(
			NewMarkupSettingKey(allowStrongSetting),
			kernel.BoolSetting(false),
		)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	beforeRevision := session.Runtime().Revision()
	beforeProjection := session.Projection().Text()
	beforeSelection := session.Selection()

	outcome, err := session.ExecuteMarkup(MarkupStrong)
	if err == nil {
		t.Fatal("strong formatting bypassed allow-strong")
	}
	if outcome != 0 {
		t.Fatalf("strong outcome = %v", outcome)
	}
	var inputError *kernel.InputError
	if !errors.As(err, &inputError) ||
		inputError.Kind != kernel.InputErrorInvalidAttributeInput ||
		inputError.Node != node ||
		inputError.Key != legacyEmphasisAttribute {
		t.Fatalf("strong error = %#v", err)
	}
	if session.Runtime().Revision() != beforeRevision ||
		session.Projection().Text() != beforeProjection ||
		session.Selection() != beforeSelection {
		t.Fatal("rejected strong formatting mutated editor state")
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.MarkCount() != 0 {
		t.Fatalf("rejected strong formatting marks = %#v", nodeValue.Marks())
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

// A selection spanning semantic text and generated markers deletes as one
// compound projected edit: attribute intents first, then the text intent,
// all validated against the same pre-write draft.
func TestSessionDeletesSelectionSpanningGeneratedMarkers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		from, to model.UTF16Offset
		markup   MarkupCommand
		before   string
	}{
		{"trailing strong", 6, 11, MarkupStrong, "hello **world**"},
		{"leading emphasis", 0, 5, MarkupEmphasis, "*hello* world"},
	}
	for _, tc := range cases {
		for _, command := range []TextCommand{TextBackspace, TextDelete} {
			session, node := newSessionFixture(t, "hello world", false)
			revision := session.Runtime().Revision()
			if err := session.Select(model.NewTextSelection(
				model.NewTextAnchor(node, tc.from, model.AffinityRight, revision),
				model.NewTextAnchor(node, tc.to, model.AffinityLeft, revision),
			)); err != nil {
				t.Fatal(err)
			}
			if _, err := session.ExecuteMarkup(tc.markup); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != tc.before {
				t.Fatalf("%s: formatted projection = %q", tc.name, got)
			}
			if outcome, err := session.ExecuteText(TextSelectAll); err != nil ||
				outcome != kernel.OutcomeHandled {
				t.Fatalf("%s: select all outcome=%v err=%v", tc.name, outcome, err)
			}
			outcome, err := session.ExecuteText(command)
			if err != nil || outcome != kernel.OutcomeHandled {
				t.Fatalf("%s %s: outcome=%v err=%v", tc.name, command.ID(), outcome, err)
			}
			nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
			if got := session.Projection().Text(); got != "" || nodeValue.Text().String() != "" {
				t.Fatalf("%s %s: projection=%q semantic=%q",
					tc.name, command.ID(), got, nodeValue.Text().String())
			}
			if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != tc.before {
				t.Fatalf("%s %s: undo projection = %q", tc.name, command.ID(), got)
			}
		}
	}
}

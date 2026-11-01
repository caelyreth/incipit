package editor

import (
	"math"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func feedbackSnapshot(
	t *testing.T,
	document *model.Document,
	root model.NodeID,
	edits ...model.TextEdit,
) (model.DocumentSnapshot, []model.TextEdit) {
	t.Helper()
	return document.Snapshot(), edits
}

func deleteEdit(node model.NodeID, start, end model.UTF16Offset) model.TextEdit {
	return model.NewTextEditReplacement(node, model.NewUTF16Range(start, end), "")
}

func TestResolveCorrectionTargetsRebasesThroughLaterEdits(t *testing.T) {
	document, root := stateDocument(t, "xaBc")
	snapshot, edits := feedbackSnapshot(
		t,
		document,
		root,
		model.NewTextEditReplacement(root, model.NewUTF16Range(1, 2), "B"),
		model.NewTextEditReplacement(root, model.EmptyUTF16Range(0), "x"),
	)
	targets := ResolveCorrectionTargets([]int{0}, edits, snapshot)
	if len(targets) != 1 {
		t.Fatalf("targets = %#v", targets)
	}
	if targets[0].Node() != root ||
		targets[0].RangeUTF16() != model.NewUTF16Range(2, 3) ||
		targets[0].Revision() != snapshot.Revision() {
		t.Fatalf("rebased target = %#v", targets[0])
	}
	if err := targets[0].Validate(snapshot.Revision(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestFeedbackRebaseUsesTheNodeThatOwnsTransferredText(t *testing.T) {
	document, root := stateDocument(t, "abc")
	var other model.NodeID
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		var err error
		other, err = draft.CreateNode(root, 0, model.NewNodeTypeID("paragraph"))
		if err != nil {
			return err
		}
		return draft.SetTextValue(other, "xyz")
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := document.Snapshot()
	target := input.NewTextTarget(root, model.NewUTF16Range(1, 2), snapshot.Revision())
	rebased, ok := rebaseInside(target, []model.TextEdit{
		model.NewTextTransfer(root, 1, other, 0, false),
	}, snapshot.Revision())
	if !ok || rebased.Node() != other ||
		rebased.RangeUTF16() != model.NewUTF16Range(0, 1) {
		t.Fatalf("transferred target = %#v, %v", rebased, ok)
	}
	if err := rebased.Validate(snapshot.Revision(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDeletionOnlyCorrectionsHighlightsNearestSurvivingGrapheme(t *testing.T) {
	tests := []struct {
		name          string
		finalText     string
		correction    model.UTF16Range
		expectedRange model.UTF16Range
		later         []model.TextEdit
	}{
		{
			name:          "previous supplementary scalar",
			finalText:     "🦀",
			correction:    model.NewUTF16Range(2, 3),
			expectedRange: model.NewUTF16Range(0, 2),
			later:         []model.TextEdit{model.NewTextEditReplacement(1, model.EmptyUTF16Range(2), "")},
		},
		{
			name:          "previous combining cluster",
			finalText:     "a\u0301",
			correction:    model.NewUTF16Range(2, 3),
			expectedRange: model.NewUTF16Range(0, 2),
			later:         []model.TextEdit{model.NewTextEditReplacement(1, model.EmptyUTF16Range(2), "")},
		},
		{
			name:          "next grapheme at document start",
			finalText:     "bc",
			correction:    model.NewUTF16Range(0, 1),
			expectedRange: model.NewUTF16Range(0, 1),
			later:         []model.TextEdit{model.NewTextEditReplacement(1, model.EmptyUTF16Range(0), "")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document, _ := stateDocument(t, test.finalText)
			snapshot := document.Snapshot()
			edits := []model.TextEdit{
				model.NewTextEditReplacement(1, test.correction, ""),
			}
			edits = append(edits, test.later...)
			targets := ResolveCorrectionTargets([]int{0}, edits, snapshot)
			if len(targets) != 1 || targets[0].RangeUTF16() != test.expectedRange {
				t.Fatalf("targets = %#v, want range %#v", targets, test.expectedRange)
			}
		})
	}
}

func TestResolveDeletionOnlyCorrectionsDeduplicatesAndSkipsInvalidIndices(t *testing.T) {
	document, _ := stateDocument(t, "a")
	snapshot := document.Snapshot()
	edits := []model.TextEdit{
		deleteEdit(1, 1, 2),
		deleteEdit(1, 1, 2),
	}
	targets := ResolveCorrectionTargets([]int{-1, 0, 1, 2, 99}, edits, snapshot)
	if len(targets) != 1 || targets[0].RangeUTF16() != model.NewUTF16Range(0, 1) {
		t.Fatalf("targets = %#v", targets)
	}
	if targets[0].Node() != snapshot.Root() || targets[0].Revision() != snapshot.Revision() {
		t.Fatalf("target identity = %#v", targets[0])
	}
}

// Pulses play side by side, each to its own end; publishing another never
// cuts one short, and an empty publication starts nothing.
func TestFeedbackPulsesPlayToTheirOwnEnds(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	clock := &fakeStateClock{now: now}
	feedback := NewFeedback(clock)
	document, root := stateDocument(t, "xy")
	snapshot := document.Snapshot()
	first := input.NewTextTarget(root, model.NewUTF16Range(0, 1), snapshot.Revision())
	second := input.NewTextTarget(root, model.NewUTF16Range(1, 2), snapshot.Revision())
	feedback.Publish([]input.TextTarget{first})
	feedback.PublishAt([]input.TextTarget{second}, now.Add(300*time.Millisecond))
	feedback.Publish(nil)
	if pulses := feedback.Pulses(now.Add(400 * time.Millisecond)); len(pulses) != 2 ||
		pulses[0].ID == pulses[1].ID {
		t.Fatalf("pulses = %#v", pulses)
	}
	if pulses := feedback.Pulses(now.Add(FeedbackPulseDuration)); len(pulses) != 1 ||
		pulses[0].Targets[0] != second {
		t.Fatalf("after the first ended: %#v", pulses)
	}
	if deadline, ok := feedback.Deadline(); !ok || !deadline.Equal(now.Add(300*time.Millisecond+FeedbackPulseDuration)) {
		t.Fatalf("deadline = %v %v", deadline, ok)
	}
	if feedback.Active(now.Add(300*time.Millisecond + FeedbackPulseDuration)) {
		t.Fatal("a pulse outlived its end")
	}
}

// A pulse taken back plays from where it was to its start, in its share of
// the rewind; played again, it goes on from there to its end.
func TestFeedbackRewindsAndReplaysFromWhereItIs(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	feedback := NewFeedback(&fakeStateClock{now: now})
	document, root := stateDocument(t, "xyz")
	revision := document.Snapshot().Revision()
	target := input.NewTextTarget(root, model.NewUTF16Range(1, 2), revision)
	feedback.Publish([]input.TextTarget{target})
	midway := now.Add(FeedbackPulseDuration / 2)
	phase, found := feedback.Take(root, model.NewUTF16Range(0, 1), midway)
	if !found || phase < 0.49 || phase > 0.51 || len(feedback.Pulses(midway)) != 0 {
		t.Fatalf("taken at %v %v", phase, found)
	}
	feedback.Rewind([]input.TextTarget{target}, phase, midway)
	pulse := feedback.Pulses(midway)[0]
	if pulse.From != phase || pulse.To != 0 ||
		pulse.Duration != time.Duration(float32(FeedbackRewindDuration)*phase) {
		t.Fatalf("rewind = %#v", pulse)
	}
	before := surface.TimedOpacity(now, FeedbackPulseDuration, 88, surface.EnvelopePulse).Sample(midway, true)
	if after := pulse.Opacity(88).Sample(midway, true); math.Abs(float64(after-before)) > 1e-4 {
		t.Fatalf("rewind starts at %v, the pulse was at %v", after, before)
	}
	back, _ := feedback.Take(root, target.RangeUTF16(), midway.Add(pulse.Duration/2))
	feedback.Replay([]input.TextTarget{target}, back, midway)
	replay := feedback.Pulses(midway)[0]
	if replay.From != back || replay.To != 1 {
		t.Fatalf("replay = %#v", replay)
	}
}

// Typing beside a pulse leaves it over its own text: its ends keep out what
// is typed against them, and it ends only with its text.
func TestFeedbackRebasesWithoutSpreading(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	feedback := NewFeedback(&fakeStateClock{now: now})
	document, root := stateDocument(t, "abc")
	revision := document.Snapshot().Revision()
	feedback.Publish([]input.TextTarget{input.NewTextTarget(root, model.NewUTF16Range(1, 2), revision)})
	feedback.Rebase([]model.TextEdit{
		model.NewTextEdit(root, model.NewUTF16Range(2, 2), 1),
		model.NewTextEdit(root, model.NewUTF16Range(1, 1), 2),
	}, revision+1)
	targets := feedback.Targets()
	if len(targets) != 1 || targets[0].RangeUTF16() != model.NewUTF16Range(3, 4) || targets[0].Revision() != revision+1 {
		t.Fatalf("rebased = %#v", targets)
	}
	feedback.Rebase([]model.TextEdit{model.NewTextEdit(root, model.NewUTF16Range(2, 5), 0)}, revision+2)
	if len(feedback.Targets()) != 0 {
		t.Fatal("a pulse outlived its text")
	}
}

func TestFeedbackPublishesResolvedCorrectionLog(t *testing.T) {
	document, _ := stateDocument(t, "a")
	snapshot := document.Snapshot()
	edits := []model.TextEdit{
		deleteEdit(1, 1, 2),
		deleteEdit(1, 1, 2),
	}
	var log CorrectionLog
	log.Record(0)
	log.Record(1)
	feedback := NewFeedback(&fakeStateClock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)})
	feedback.PublishCorrections(log, edits, snapshot)
	targets := feedback.Targets()
	if len(targets) != 1 || targets[0].RangeUTF16() != model.NewUTF16Range(0, 1) {
		t.Fatalf("resolved targets = %#v", targets)
	}
	copied := feedback.Targets()
	copied[0] = input.NewTextTarget(99, model.NewUTF16Range(0, 1), 99)
	if feedback.Targets()[0].Node() != targets[0].Node() {
		t.Fatal("feedback exposed mutable target storage")
	}
}

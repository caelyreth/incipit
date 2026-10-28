package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
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

func TestFeedbackUsesOneGlobal1100MillisecondPulse(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	clock := &fakeStateClock{now: now}
	feedback := NewFeedback(clock)
	document, root := stateDocument(t, "x")
	snapshot := document.Snapshot()
	target := input.NewTextTarget(root, model.NewUTF16Range(0, 1), snapshot.Revision())
	feedback.Publish([]input.TextTarget{target})
	if started, ok := feedback.Started(); !ok || started != now {
		t.Fatalf("started = %v %v", started, ok)
	}
	if !feedback.Active(now.Add(1099 * time.Millisecond)) {
		t.Fatal("pulse ended before 1100ms")
	}
	if feedback.Active(now.Add(1100 * time.Millisecond)) {
		t.Fatal("pulse survived its exact deadline")
	}
	if deadline, ok := feedback.Deadline(); !ok || deadline != now.Add(FeedbackPulseDuration) {
		t.Fatalf("deadline = %v %v", deadline, ok)
	}

	later := now.Add(300 * time.Millisecond)
	feedback.PublishAt([]input.TextTarget{target}, later)
	if started, _ := feedback.Started(); started != later {
		t.Fatalf("republication did not reset pulse: %v", started)
	}
	feedback.Publish(nil)
	if _, ok := feedback.Started(); ok {
		t.Fatal("empty publication retained pulse")
	}
	if len(feedback.Targets()) != 0 {
		t.Fatal("empty publication retained targets")
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

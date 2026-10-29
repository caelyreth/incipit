package editor

import (
	"math"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func feedbackFills(t *testing.T, session *Session, now time.Time) []surface.Decoration {
	t.Helper()
	frame, err := session.SurfaceFrame(now, nil)
	if err != nil {
		t.Fatal(err)
	}
	fills := make([]surface.Decoration, 0)
	for _, decoration := range frame.Decorations() {
		if decoration.ID.Producer == "feedback" {
			fills = append(fills, decoration)
		}
	}
	return fills
}

func typeEach(t *testing.T, session *Session, text string) {
	t.Helper()
	for _, r := range text {
		typeSession(t, session, string(r))
	}
}

func advance(session *Session, by time.Duration) time.Time {
	clock := session.feedback.clock.(*sessionClock)
	clock.now = clock.now.Add(by)
	return clock.now
}

// A refinement that rewrites typed text pulses over what it wrote, and the
// pulse plays to its end: typing on beside it neither cuts it nor spreads it.
func TestRefinementPulsePlaysThroughTyping(t *testing.T) {
	session, node := newSessionFixture(t, "", false)
	now := session.feedback.clock.Now()
	typeEach(t, session, "Wait...")
	value, _ := session.Runtime().Snapshot().Document().Node(node)
	if value.Text().String() != "Wait…" {
		t.Fatalf("typography wrote %q", value.Text().String())
	}
	fills := feedbackFills(t, session, now)
	if len(fills) != 1 {
		t.Fatalf("feedback decorations = %#v", fills)
	}
	pulse := fills[0]
	if pulse.Primitive.Kind != surface.PrimitiveFill || pulse.Opacity.Kind != surface.OpacityTimed ||
		pulse.Opacity.Envelope != surface.EnvelopePulse {
		t.Fatalf("pulse = %#v", pulse)
	}
	if got := pulse.Primitive.Ranges; len(got) != 1 || got[0] != model.NewUTF16Range(4, 5) {
		t.Fatalf("pulse covers %#v, want the ellipsis", got)
	}

	advance(session, FeedbackPulseDuration/4)
	typeEach(t, session, " ok")
	later := feedbackFills(t, session, now.Add(FeedbackPulseDuration/2))
	if len(later) != 1 || later[0].ID != pulse.ID ||
		later[0].Primitive.Ranges[0] != model.NewUTF16Range(4, 5) ||
		later[0].Opacity.Started != pulse.Opacity.Started {
		t.Fatalf("typing changed the pulse: %#v", later)
	}
	if later[0].Sample(now.Add(FeedbackPulseDuration/2), true) <= 0 {
		t.Fatal("the pulse stopped showing midway")
	}
	if len(feedbackFills(t, session, now.Add(FeedbackPulseDuration))) != 0 {
		t.Fatal("the pulse outlived its duration")
	}
}

// Consecutive corrections each pulse under their own identity, together.
func TestEachCorrectionStartsItsOwnPulse(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	now := session.feedback.clock.Now()
	typeEach(t, session, "a...")
	first := feedbackFills(t, session, now)
	typeEach(t, session, " b...")
	second := feedbackFills(t, session, now)
	if len(first) != 1 || len(second) != 2 || second[0].ID != first[0].ID || second[1].ID == first[0].ID {
		t.Fatalf("pulses %#v then %#v, want the first kept and a second of its own", first, second)
	}
}

// Undoing a correction plays its pulse back over the text it restores, from
// exactly where it was, so the highlight recedes rather than vanishing; redo
// plays it on again from there.
func TestUndoRewindsACorrectionPulseFromWhereItIs(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeEach(t, session, "Wait...")
	midway := advance(session, FeedbackPulseDuration/3)
	before := feedbackFills(t, session, midway)[0].Sample(midway, true)

	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "Wait.." {
		t.Fatalf("undo left %q", got)
	}
	rewind := feedbackFills(t, session, midway)
	if len(rewind) != 1 || rewind[0].Primitive.Ranges[0] != model.NewUTF16Range(4, 6) {
		t.Fatalf("rewind = %#v", rewind)
	}
	if after := rewind[0].Sample(midway, true); math.Abs(float64(after-before)) > 1e-3 {
		t.Fatalf("undo jumped the pulse from %v to %v", before, after)
	}
	deadline, _ := rewind[0].Deadline()
	if !deadline.Before(midway.Add(FeedbackRewindDuration)) || rewind[0].Sample(deadline, true) != 0 {
		t.Fatalf("rewind ends at %v", deadline)
	}

	backAt := midway.Add(deadline.Sub(midway) / 2)
	session.feedback.clock.(*sessionClock).now = backAt
	reached := rewind[0].Sample(backAt, true)
	if _, err := session.ExecuteHistory(HistoryRedo); err != nil {
		t.Fatal(err)
	}
	replay := feedbackFills(t, session, backAt)
	if len(replay) != 1 || replay[0].Primitive.Ranges[0] != model.NewUTF16Range(4, 5) {
		t.Fatalf("replay = %#v", replay)
	}
	if again := replay[0].Sample(backAt, true); math.Abs(float64(again-reached)) > 1e-3 {
		t.Fatalf("redo jumped the pulse from %v to %v", reached, again)
	}
}

// A correction long since shown still plays back when undone: the restored
// text warms and cools, as the correction did, in reverse.
func TestUndoAfterThePulsePlaysItBackWhole(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeEach(t, session, "Wait...")
	later := advance(session, 2*FeedbackPulseDuration)
	if len(feedbackFills(t, session, later)) != 0 {
		t.Fatal("pulse still playing")
	}
	if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	rewind := feedbackFills(t, session, later)
	if len(rewind) != 1 || rewind[0].Opacity.From != 1 || rewind[0].Opacity.To != 0 ||
		rewind[0].Opacity.Duration != FeedbackRewindDuration {
		t.Fatalf("whole rewind = %#v", rewind)
	}
	if rewind[0].Sample(later, true) != 0 ||
		rewind[0].Sample(later.Add(FeedbackRewindDuration/2), true) <= 0 {
		t.Fatal("the rewind does not rise from nothing")
	}

	// Undoing a step that made no correction plays nothing.
	session2, _ := newSessionFixture(t, "", false)
	typeEach(t, session2, "plain")
	if _, err := session2.ExecuteHistory(HistoryUndo); err != nil {
		t.Fatal(err)
	}
	if len(feedbackFills(t, session2, session2.feedback.clock.Now())) != 0 {
		t.Fatal("an uncorrected undo pulsed")
	}
}

// Refinement inside emphasis stays emphasized, as the typing it refines
// would have.
func TestRefinementInsideEmphasisStaysInside(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeEach(t, session, "*Wait...")
	if got := shown(session); got != "*Wait…|*" {
		t.Fatalf("refined emphasis shows %s, want the ellipsis inside", got)
	}
}

func TestCJKRefinementPulsesToo(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(CJKPolicyID, true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	typeEach(t, session, "中文,")
	if len(session.Feedback().Targets()) == 0 {
		t.Fatal("CJK punctuation refinement left no feedback")
	}
}

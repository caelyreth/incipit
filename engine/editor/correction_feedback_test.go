package editor

import (
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

// A refinement that rewrites typed text pulses over what it wrote, once,
// and the pulse ends on its own or with the next edit.
func TestRefinementPulsesOverTheTextItRewrote(t *testing.T) {
	session, node := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeEach(t, session, "Wait...")
	value, _ := session.Runtime().Snapshot().Document().Node(node)
	if value.Text().String() != "Wait…" {
		t.Fatalf("typography wrote %q", value.Text().String())
	}
	targets := session.Feedback().Targets()
	if len(targets) != 1 || targets[0].RangeUTF16() != model.NewUTF16Range(4, 5) {
		t.Fatalf("feedback targets = %#v, want the ellipsis", targets)
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
	if mid := pulse.Sample(now.Add(FeedbackPulseDuration/4), true); mid <= 0 {
		t.Fatalf("pulse at a quarter = %v, want visible", mid)
	}
	if len(feedbackFills(t, session, now.Add(FeedbackPulseDuration))) != 0 {
		t.Fatal("the pulse outlived its duration")
	}

	typeSession(t, session, "x")
	if len(session.Feedback().Targets()) != 0 || len(feedbackFills(t, session, now)) != 0 {
		t.Fatal("an ordinary edit kept the previous correction's pulse")
	}
}

// Consecutive corrections each get a pulse of their own identity, so the
// second starts fresh instead of inheriting the first one's fade.
func TestEachCorrectionStartsItsOwnPulse(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeEach(t, session, "a...")
	first := feedbackFills(t, session, now)
	typeEach(t, session, " b...")
	second := feedbackFills(t, session, now)
	if len(first) != 1 || len(second) != 1 || first[0].ID == second[0].ID {
		t.Fatalf("pulses %#v then %#v, want distinct identities", first, second)
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

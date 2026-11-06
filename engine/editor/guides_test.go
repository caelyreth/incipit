package editor

import (
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/surface"
)

func keyNames(target kernel.BindingTarget) []string {
	return []string{target.String()}
}

func frameGuides(t *testing.T, session *Session, now time.Time, keys GuideKeys) []surface.Decoration {
	t.Helper()
	frame, err := session.SurfaceFrame(now, keys)
	if err != nil {
		t.Fatal(err)
	}
	guides := make([]surface.Decoration, 0)
	for _, decoration := range frame.Decorations() {
		if decoration.Primitive.Kind == surface.PrimitiveGuide {
			guides = append(guides, decoration)
		}
	}
	return guides
}

func TestGuideMarksWhereTabLandsPastGeneratedCloser(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abcd", true)
	now := time.Unix(1_700_000_000, 0)
	if err := session.SelectProjected(3, 3); err != nil {
		t.Fatal(err)
	}
	guides := frameGuides(t, session, now, keyNames)
	if len(guides) != 1 {
		t.Fatalf("guides = %#v", guides)
	}
	guide := guides[0]
	binding := session.Runtime().Configuration().BindingsFor(LeaveCommandID)[0]
	if !slices.Equal(guide.Primitive.Keys, []string{binding.String()}) {
		t.Fatalf("keys = %q, want the configured %q", guide.Primitive.Keys, binding.String())
	}
	if guide.Primitive.At != 5 { // a*bc*|d
		t.Fatalf("guide at %d, want past the generated closer", guide.Primitive.At)
	}
	if guide.ID.Producer != surface.ProducerID(NavigationPolicyID) || guide.ExpiresAt != nil {
		t.Fatalf("guide = %#v", guide)
	}
	if outcome, err := session.pressLeave(now); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("tab outcome=%v err=%v", outcome, err)
	}
	if got := session.Caret().RangeUTF16().Start; got != guide.Primitive.At {
		t.Fatalf("Tab landed at %d, guide promised %d", got, guide.Primitive.At)
	}
}

func TestGuideFollowsAPairCloserAsTextGrowsInside(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeSession(t, session, "(")
	typeSession(t, session, "x")
	before := frameGuides(t, session, now, keyNames)
	typeSession(t, session, "y")
	after := frameGuides(t, session, now, keyNames)
	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("guides before=%d after=%d", len(before), len(after))
	}
	if before[0].Primitive.At != 3 || after[0].Primitive.At != 4 {
		t.Fatalf("guide at %d then %d, want past the closer", before[0].Primitive.At, after[0].Primitive.At)
	}
	if before[0].ID != after[0].ID {
		t.Fatal("a moving destination must keep one identity so it can glide")
	}
}

func TestGuideFollowsTheCommandBinding(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abcd", true)
	now := time.Unix(1_700_000_000, 0)
	if err := session.SelectProjected(3, 3); err != nil {
		t.Fatal(err)
	}
	original := session.Runtime().Configuration().BindingsFor(LeaveCommandID)[0]
	remapped := kernel.NewBindingTarget(original.Context(), "j", kernel.ModifiersControl)
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.Remap(LeaveCommandID, remapped)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	guides := frameGuides(t, session, now, keyNames)
	if len(guides) != 1 || !slices.Equal(guides[0].Primitive.Keys, []string{remapped.String()}) {
		t.Fatalf("remapped guides = %#v", guides)
	}
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.Unbind(LeaveCommandID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if guides := frameGuides(t, session, now, keyNames); len(guides) != 0 {
		t.Fatalf("an unbound command still shows %#v", guides)
	}
}

func TestGuidesHideWithoutNotationSelectionOrPolicy(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abcd", true)
	now := time.Unix(1_700_000_000, 0)
	if err := session.SelectProjected(3, 3); err != nil {
		t.Fatal(err)
	}
	if guides := frameGuides(t, session, now, nil); len(guides) != 0 {
		t.Fatalf("no notation, guides = %#v", guides)
	}
	if guides := frameGuides(t, session, now, func(kernel.BindingTarget) []string { return nil }); len(guides) != 0 {
		t.Fatalf("empty notation, guides = %#v", guides)
	}
	if err := session.SelectProjected(2, 3); err != nil {
		t.Fatal(err)
	}
	if guides := frameGuides(t, session, now, keyNames); len(guides) != 0 {
		t.Fatalf("selection shows guides %#v", guides)
	}
	if err := session.SelectProjected(3, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetSetting(
			kernel.NewSettingKey(NavigationPolicyID, kernel.NewSettingID("emphasis")),
			kernel.BoolSetting(false),
		)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if guides := frameGuides(t, session, now, keyNames); len(guides) != 0 {
		t.Fatalf("disabled emphasis navigation shows %#v", guides)
	}
}

func TestReturnGuideExpiresWithItsOffer(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeSession(t, session, "a long enough edit")
	if err := session.SelectPointer(0, 0, now); err != nil {
		t.Fatal(err)
	}
	guides := frameGuides(t, session, now, keyNames)
	if len(guides) != 1 || guides[0].Primitive.At != 18 || guides[0].ExpiresAt == nil {
		t.Fatalf("return guides = %#v", guides)
	}
	if deadline, ok := guides[0].Deadline(); !ok || !deadline.Equal(now.Add(14*time.Second)) {
		t.Fatalf("deadline = %v %v", deadline, ok)
	}
	if guides := frameGuides(t, session, now.Add(15*time.Second), keyNames); len(guides) != 0 {
		t.Fatalf("expired return still guides %#v", guides)
	}
}

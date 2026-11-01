package editor

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func TestSessionTextSpanAndUnifiedCommandDispatch(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcd", true)
	span, err := session.TextSpan()
	if err != nil {
		t.Fatal(err)
	}
	if span != model.NewUTF16Range(0, 6) {
		t.Fatalf("text span = %#v", span)
	}
	if outcome, err := session.ExecuteCommand(TextDocumentEnd.ID(), time.Now()); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("command outcome=%v err=%v", outcome, err)
	}
	if session.Selection().Head().OffsetUTF16() != 4 {
		t.Fatalf("command head = %d", session.Selection().Head().OffsetUTF16())
	}
	if len(session.Runtime().Configuration().BindingsFor(TabCommand().ID)) == 0 {
		t.Fatal("Tab binding missing")
	}
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	_ = session
}

func TestSessionTabLandsAfterGeneratedMarkup(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcd", true)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 2, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	offer, ok, err := session.TabOffer(time.Unix(1_700_000_000, 0))
	if err != nil || !ok {
		t.Fatalf("offer=%#v ok=%v err=%v", offer, ok, err)
	}
	if offer.Kind != TabEmphasis || !offer.HasMark {
		t.Fatalf("offer = %#v", offer)
	}
	outcome, err := session.Tab(time.Unix(1_700_000_000, 0))
	if err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("tab outcome=%v err=%v", outcome, err)
	}
	want := model.UTF16Offset(0)
	for _, segment := range session.Projection().Segments() {
		if target, ok := segment.AttributeTarget(); ok &&
			MarkerKeyIdentifies(target.Key(), offer.Mark) {
			want = segment.ProjectedRange().End
		}
	}
	if session.Caret().RangeUTF16().Start != want {
		t.Fatalf("caret=%d want generated closer end %d", session.Caret().RangeUTF16().Start, want)
	}
}

func TestSessionPointerSelectsProjectedPosition(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcdef", false)
	now := time.Unix(1_700_000_000, 0)
	if err := session.SelectPointer(1, 1, now); err != nil {
		t.Fatal(err)
	}
	if session.Selection().Head().OffsetUTF16() != 1 || session.Selection().Node() != node {
		t.Fatalf("selection = %#v", session.Selection())
	}
}

func TestSessionCommandDispatchRejectsUnknownCommands(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	outcome, err := session.ExecuteCommand("missing.command", time.Unix(1_700_000_000, 0))
	if err != nil || outcome != kernel.OutcomePass {
		t.Fatalf("unknown command outcome=%v err=%v", outcome, err)
	}
}

func TestSessionOffersNoTabWhileMarkerNotationIsSelected(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	if _, err := session.ExecuteMarkup(MarkupEmphasis); err != nil {
		t.Fatal(err)
	}
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	if session.Presentation().VisualCollapsed() {
		t.Fatal("marker-only selection reported visually collapsed")
	}
	if _, ok, err := session.TabOffer(time.Unix(1_700_000_000, 0)); err != nil || ok {
		t.Fatalf("Tab offer while marker selected: ok=%v err=%v", ok, err)
	}
}

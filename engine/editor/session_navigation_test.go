package editor

import (
	"errors"
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
	if outcome, err := session.DoCommand(DocumentEndCommandID, time.Now()); err != nil ||
		outcome.Outcome != kernel.OutcomeHandled {
		t.Fatalf("command outcome=%v err=%v", outcome, err)
	}
	if session.Selection().Head().OffsetUTF16() != 4 {
		t.Fatalf("command head = %d", session.Selection().Head().OffsetUTF16())
	}
	if len(session.Runtime().Configuration().BindingsFor(LeaveCommandID)) == 0 {
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
	if places := session.innerPlaces(); len(places) != 1 || places[0].Kind != PlaceEmphasis {
		t.Fatalf("places = %#v", places)
	}
	block, _ := session.Runtime().Snapshot().Document().Node(node)
	mark := block.Marks()[0].ID()
	outcome, err := session.pressLeave(time.Unix(1_700_000_000, 0))
	if err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("tab outcome=%v err=%v", outcome, err)
	}
	want := model.UTF16Offset(0)
	for _, segment := range session.Projection().Segments() {
		if target, ok := segment.AttributeTarget(); ok &&
			MarkerKeyIdentifies(target.Key(), mark) {
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

func TestDismissSelectionCollapsesToItsVisualHead(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abcdef", false)
	if err := session.SelectProjected(4, 1); err != nil {
		t.Fatal(err)
	}
	outcome, err := session.DoCommand(
		DismissCommandID,
		time.Unix(1_700_000_000, 0),
	)
	if err != nil || outcome.Outcome != kernel.OutcomeHandled {
		t.Fatalf("dismiss outcome=%v err=%v", outcome, err)
	}
	if visual, err := session.VisualSelection(); err != nil ||
		visual != model.EmptyUTF16Range(1) {
		t.Fatalf("visual selection = %v, %v", visual, err)
	}
	if got := session.Projection().Text(); got != "abcdef" {
		t.Fatalf("dismiss changed text to %q", got)
	}
}

func TestSessionCommandDispatchRejectsUnknownCommands(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	outcome, err := session.DoCommand("missing.command", time.Unix(1_700_000_000, 0))
	if !errors.Is(err, ErrUnknownCommand) || outcome.Outcome != kernel.OutcomePass {
		t.Fatalf("unknown command outcome=%v err=%v", outcome, err)
	}
}

func TestSessionOffersNoTabWhileMarkerNotationIsSelected(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "abc", false)
	if _, err := session.doFormat(markupEmphasis); err != nil {
		t.Fatal(err)
	}
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	if session.Presentation().VisualCollapsed() {
		t.Fatal("marker-only selection reported visually collapsed")
	}
	if next, ok := session.stepFor(Leave, time.Unix(1_700_000_000, 0)); ok {
		t.Fatalf("Leave step while marker selected: %#v", next)
	}
}

func TestUnifiedDispatchIncludesNestingCommands(t *testing.T) {
	t.Parallel()
	document, blocks, err := NewBlockDocument("one\ntwo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for _, block := range blocks {
			if err := shapeBlock(draft, block, blockShape{kind: BulletNode}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, document, blocks[0])
	revision := session.Runtime().Revision()
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(blocks[1], 0, model.AffinityRight, revision),
		model.NewTextAnchor(blocks[1], 3, model.AffinityLeft, revision),
	)); err != nil {
		t.Fatal(err)
	}
	outcome, err := session.DoCommand(NestDeeperCommandID, time.Unix(1_700_000_000, 0))
	if err != nil || outcome.Outcome != kernel.OutcomeHandled {
		t.Fatalf("nest outcome=%v err=%v", outcome, err)
	}
	if got := session.Projection().Text(); got != "- one\n  - two" {
		t.Fatalf("projection = %q", got)
	}
}

func TestWordCommandsTreatAbsentTextAsEmpty(t *testing.T) {
	t.Parallel()
	document, node := emptyBlockDocument(t)
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.ClearText(node)
	}); err != nil {
		t.Fatal(err)
	}
	session := sessionFor(t, document, node)
	if outcome, err := session.doWord(wordNext); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("word outcome=%v err=%v", outcome, err)
	}
}

// Leave pops exactly one place at a time, innermost first, and Return goes
// back into the one just left.
func TestLeaveIsLastInFirstOutAndReturnReenters(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeKeys(t, session, "(a[b")
	text := session.Presentation().Flow.Text()
	inner := session.Caret().RangeUTF16().Start
	if places := session.innerPlaces(); len(places) != 2 || places[0].Kind != PlacePair || places[1].Kind != PlacePair {
		t.Fatalf("places in %q = %#v", text, places)
	}
	if _, err := session.pressLeave(now); err != nil {
		t.Fatal(err)
	}
	once := session.Caret().RangeUTF16().Start
	if once != inner+1 {
		t.Fatalf("first Leave in %q landed at %d, want %d", text, once, inner+1)
	}
	if places := session.innerPlaces(); len(places) != 1 {
		t.Fatalf("after one Leave, places = %#v", places)
	}
	if _, err := session.pressReturn(); err != nil {
		t.Fatal(err)
	}
	if got := session.Caret().RangeUTF16().Start; got != inner {
		t.Fatalf("Return landed at %d, want back at %d", got, inner)
	}
	for range 2 {
		if _, err := session.pressLeave(now); err != nil {
			t.Fatal(err)
		}
	}
	if got := session.Caret().RangeUTF16().Start; got != inner+2 {
		t.Fatalf("two Leaves landed at %d in %q, want %d", got, text, inner+2)
	}
	if outcome, err := session.pressLeave(now); err != nil || outcome != kernel.OutcomePass {
		t.Fatalf("Leave with nothing to leave = %v, %v", outcome, err)
	}
}

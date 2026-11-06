package editor

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Tab just after an inline formula goes into its source at its end, where
// its guide says it lands; in its source a pair, narrower, goes first.
func TestTabGoesIntoTheFormulaBeforeTheCaret(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeKeys(t, session, "a $x^2")
	text := session.Presentation().Flow.Text()
	end := model.UTF16Offset(strings.LastIndex(text, "$") + 1)
	if err := session.SelectProjected(end, end); err != nil {
		t.Fatal(err)
	}
	guides := frameGuides(t, session, now, keyNames)
	if len(guides) != 1 || guides[0].Primitive.At != end-1 {
		t.Fatalf("guides after %q = %#v", text, guides)
	}
	if outcome, err := session.leave(now); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("tab outcome=%v err=%v", outcome, err)
	}
	if got := session.Caret().RangeUTF16().Start; got != end-1 {
		t.Fatalf("Tab landed at %d, its guide at %d", got, end-1)
	}
	typeKeys(t, session, "+(")
	offer, ok, err := session.TabOffer(now)
	if err != nil || !ok || offer.Kind != TabPair {
		t.Fatalf("in a pair in the source, the offer is %#v, %v, %v", offer, ok, err)
	}
}

// In a formula block's source Tab leaves it for the line after it, as its
// guide says.
func TestTabLeavesAFormulaBlock(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	now := time.Unix(1_700_000_000, 0)
	typeKeys(t, session, "$$\\frac12\n\nafter")
	text := session.Presentation().Flow.Text()
	inside := model.UTF16Offset(strings.Index(text, "12"))
	if err := session.SelectProjected(inside, inside); err != nil {
		t.Fatal(err)
	}
	offer, ok, err := session.TabOffer(now)
	if err != nil || !ok || offer.Kind != TabFormula {
		t.Fatalf("offer in %q = %#v, %v, %v", text, offer, ok, err)
	}
	want := model.UTF16Offset(strings.Index(text, "after"))
	guides := frameGuides(t, session, now, keyNames)
	if len(guides) != 1 || guides[0].Primitive.At != want {
		t.Fatalf("guides = %#v, want at %d", guides, want)
	}
	if _, err := session.leave(now); err != nil {
		t.Fatal(err)
	}
	if got := session.Caret().RangeUTF16().Start; got != want {
		t.Fatalf("Tab landed at %d in %q, want %d", got, text, want)
	}
}

// Escape in a formula's source leaves it: after an inline one; the line
// after a block, or the one before it at the page's end.
func TestEscapeLeavesAFormula(t *testing.T) {
	escape := func(t *testing.T, session *Session) model.UTF16Offset {
		t.Helper()
		if _, err := session.DoCommand(DismissCommandID, time.Unix(1_700_000_000, 0)); err != nil {
			t.Fatal(err)
		}
		return session.Caret().RangeUTF16().Start
	}
	t.Run("inline", func(t *testing.T) {
		session, _ := newSessionFixture(t, "", false)
		typeKeys(t, session, "a $x")
		text := session.Presentation().Flow.Text()
		if got, want := escape(t, session), model.UTF16Offset(strings.LastIndex(text, "$")+1); got != want {
			t.Fatalf("Escape in %q went to %d, want %d", text, got, want)
		}
	})
	t.Run("block at the end", func(t *testing.T) {
		session, _ := newSessionFixture(t, "", false)
		typeKeys(t, session, "p\n$$x")
		text := session.Presentation().Flow.Text()
		if got, want := escape(t, session), model.UTF16Offset(strings.Index(text, "$$")-1); got != want {
			t.Fatalf("Escape in %q went to %d, want %d", text, got, want)
		}
	})
}

// A formula is edited while an end of the selection is in its source,
// either end of the source included; beside its notation, or a selection
// over all of it, it is not.
func TestAFormulaIsEditedWithASelectionEndInItsSource(t *testing.T) {
	inline := surface.Formula{Node: 1, Range: model.NewUTF16Range(2, 5), Text: model.NewUTF16Range(3, 4), Source: "x"}
	other := surface.Formula{Node: 1, Range: model.NewUTF16Range(8, 11), Text: model.NewUTF16Range(9, 10), Source: "y"}
	text := "a $x$ b $y$"
	flow, err := surface.NewFlow(projection.NewVersion(1, 1), text, []surface.StyleRun{{Bytes: model.NewByteRange(0, model.ByteOffset(len(text)))}}, []surface.Block{{Node: 1, Range: model.NewUTF16Range(0, 11)}})
	if err == nil {
		flow, err = flow.WithObjects([]surface.Object{inline, other})
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		selected model.UTF16Range
		want     []model.UTF16Range
	}{
		{model.EmptyUTF16Range(2), nil},
		{model.EmptyUTF16Range(5), nil},
		{model.NewUTF16Range(0, 7), nil},
		{model.EmptyUTF16Range(3), []model.UTF16Range{inline.Range}},
		{model.EmptyUTF16Range(4), []model.UTF16Range{inline.Range}},
		{model.NewUTF16Range(0, 3), []model.UTF16Range{inline.Range}},
		{model.NewUTF16Range(3, 4), []model.UTF16Range{inline.Range}},
		{model.NewUTF16Range(4, 10), []model.UTF16Range{inline.Range, other.Range}},
	} {
		if got := editedFormulas(flow, test.selected); !slices.Equal(got, test.want) {
			t.Errorf("selecting %v edits %v, want %v", test.selected, got, test.want)
		}
	}
}

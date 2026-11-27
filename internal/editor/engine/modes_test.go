package engine_test

import (
	"strings"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// look sets the word the looking policy's view looks for.
type look struct{ word string }

var intentLook = vocab.NewIntentKind("Look")

func (look) Kind() vocab.IntentKind { return intentLook }

// seen is the kind of the looking policy's facts.
var seen *policy.FactKind[struct{}]

// looking marks where its view's mode, a word, stands in the text.
var looking = policy.Define("looking", func(p *policy.P) {
	seen = policy.Facts[struct{}](p, "seen")
	p.Analyze(policy.Lines, func(u *policy.Unit, e *policy.Emit) {
		word, ok := u.Mode.(string)
		if !ok {
			return
		}
		text := u.Text()
		for at := 0; ; {
			i := strings.Index(text[at:], word)
			if i < 0 {
				return
			}
			e.Fact(seen.Span(u.From+at+i, u.From+at+i+len(word), struct{}{}))
			at += i + len(word)
		}
	}, []policy.FactID{seen.Kind()})
	policy.On(p, "look", policy.Intent[look](), policy.Slot.Inside(), func(c *policy.Ctx, in look, _ policy.At[doc.SlotRef]) policy.Result {
		if in.word == "" {
			return policy.Result{DropMode: true}
		}
		return policy.Result{Mode: in.word}
	}).Any()
})

func TestAViewsModeIsWhatItsPolicysFactsFollow(t *testing.T) {
	h := newHarness(t, looking)
	h.typeText("tide and tide")
	count := func() int {
		v, _ := h.e.View(1)
		ref := doc.SlotRef{Node: h.e.Snapshot().Root().Slot(0).Children()[0].ID()}
		return len(h.e.Program().FactsIn(h.e.Store(), h.e.Snapshot(), &v, ref, seen.Kind()))
	}
	if count() != 0 {
		t.Fatal("facts with no mode")
	}
	h.do(look{word: "tide"})
	if v, _ := h.e.View(1); v.Modes.Of("looking") != "tide" || count() != 2 {
		t.Fatalf("mode %v facts %d", v.Modes.Of("looking"), count())
	}
	h.do(look{word: "and"})
	if count() != 1 {
		t.Fatalf("a new mode reads again: %d", count())
	}
	h.do(look{})
	if v, _ := h.e.View(1); v.Modes.Of("looking") != nil || count() != 0 {
		t.Fatal("the mode did not drop")
	}
}

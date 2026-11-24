package engine_test

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// breakIt is an intent whose edit panics, as a broken rule's would.
type breakIt struct{}

var intentBreak = vocab.NewIntentKind("Break")

func (breakIt) Kind() vocab.IntentKind { return intentBreak }

var breaking = policy.Define("breaking", func(p *policy.P) {
	policy.On(p, "break", policy.Intent[breakIt](), policy.Slot.Inside(), func(c *policy.Ctx, _ breakIt, _ policy.At[doc.SlotRef]) policy.Result {
		return policy.Result{Edits: []vocab.Edit{func(*doc.Tx) error { panic("broken") }}}
	}).Any()
})

// TestABrokenRuleFailsItsTransactionAlone turns a rule's panic, previewed
// or done, into ErrFault: nothing of it is kept, the engine goes on.
func TestABrokenRuleFailsItsTransactionAlone(t *testing.T) {
	h := newHarness(t, breaking)
	h.typeText("tide")
	before := h.e.Snapshot()
	if _, err := h.e.Preview(1, vocab.Input{Intent: breakIt{}, At: h.now}); !errors.Is(err, engine.ErrFault) {
		t.Fatalf("preview: %v, want ErrFault", err)
	}
	if _, err := h.e.Do(1, vocab.Input{Intent: breakIt{}, At: h.now}); !errors.Is(err, engine.ErrFault) {
		t.Fatalf("do: %v, want ErrFault", err)
	}
	if h.e.Snapshot() != before {
		t.Fatal("a broken transaction changed the page")
	}
	h.typeText("s")
	if got := h.e.Snapshot().Root().Slot(0).Children()[0].Slot(0).Content().Text(); got != "tides" {
		t.Fatalf("after the fault the page reads %q", got)
	}
}

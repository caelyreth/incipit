// Package history is the history policy: typing groups, undo and redo,
// and undoing only an entry's top layer.
package history

import (
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Window is the typing group window in milliseconds.
var Window = policy.Int("typingWindow", 750, 0, 5000, policy.Label("settings.history.typingWindow"), policy.Description("settings.history.typingWindow.description"))

// Policy is the history policy.
var Policy = policy.Define("history", func(p *policy.P) {
	window := time.Duration(policy.Get(p, Window)) * time.Millisecond
	p.Grouping(func(prev, next *doc.Entry) bool { return typingGroup(prev, next, window) })
	policy.On(p, "undo", policy.Intent[vocab.Undo](), policy.Slot.Inside(), func(c *policy.Ctx, in vocab.Undo, _ policy.At[doc.SlotRef]) policy.Result {
		return policy.Result{History: &policy.HistoryRequest{Action: policy.HistoryUndo, TopLayer: in.TopLayer},
			EndComposition: c.Composition() != nil, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
	}).Any()
	policy.On(p, "redo", policy.Intent[vocab.Redo](), policy.Slot.Inside(), func(c *policy.Ctx, _ vocab.Redo, _ policy.At[doc.SlotRef]) policy.Result {
		return policy.Result{History: &policy.HistoryRequest{Action: policy.HistoryRedo},
			EndComposition: c.Composition() != nil, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
	}).Any()
	p.Command("history.undoCorrection", "commands.undoCorrection", vocab.Undo{TopLayer: true})

	// The card's demonstration, played by the engine itself.
	p.Showcase(2400*time.Millisecond,
		policy.Types("A first draft, then a second thought."),
		policy.Waits(500*time.Millisecond),
		policy.Presses(vocab.Undo{}),
		policy.Waits(450*time.Millisecond),
		policy.Presses(vocab.Undo{}),
		policy.Waits(700*time.Millisecond),
		policy.Presses(vocab.Redo{}),
		policy.Waits(450*time.Millisecond),
		policy.Presses(vocab.Redo{}),
	)
}, policy.Label("policies.history"), policy.Description("policies.history.description"))

// typingGroup reports whether next continues prev's typing: both are
// typing in one view and slot, within window of each other, typing on at
// the end or deleting back from it.
func typingGroup(prev, next *doc.Entry, window time.Duration) bool {
	if prev == nil || next == nil || prev.Class != doc.OriginTyping || next.Class != doc.OriginTyping ||
		prev.View != next.View || next.Time.Sub(prev.Time) > window || next.Time.Before(prev.Time) {
		return false
	}
	ps, _, pto, ok1 := typed(prev)
	ns, nfrom, nto, ok2 := typed(next)
	if !ok1 || !ok2 || ps != ns {
		return false
	}
	return nfrom == pto || nto == pto || nfrom == pto-1
}

// typed is the slot and extent of an entry's last text change.
func typed(e *doc.Entry) (doc.SlotRef, int, int, bool) {
	steps := e.Steps()
	for i := len(steps) - 1; i >= 0; i-- {
		for _, r := range steps[i].Records() {
			if r.Kind == doc.RecordTextInserted || r.Kind == doc.RecordTextDeleted {
				return r.Slot, r.From, r.To, true
			}
		}
	}
	return doc.SlotRef{}, 0, 0, false
}

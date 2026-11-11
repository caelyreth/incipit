// Package history is the history policy: typing groups, undo and redo,
// and undoing only an entry's top layer.
package history

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Window is the typing group window in milliseconds.
var Window = policy.Int("typingWindow", 750, 0, 5000, policy.Label("settings.history.typingWindow"), policy.Description("settings.history.typingWindow.description"))

// Policy is the history policy.
var Policy = policy.Define("history", func(p *policy.P) {
	window := time.Duration(policy.Get(p, Window)) * time.Millisecond
	p.Grouping(func(prev, next *doc.Entry) bool { return doc.CanGroup(prev, next, window) })
	policy.On(p, "undo", policy.Intent[vocab.Undo](), policy.Slot.Inside(), func(c *policy.Ctx, in vocab.Undo, _ policy.At[doc.SlotRef]) policy.Result {
		return policy.Result{History: &policy.HistoryRequest{Action: policy.HistoryUndo, TopLayer: in.TopLayer},
			EndComposition: c.Composition() != nil, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
	}).Any()
	policy.On(p, "redo", policy.Intent[vocab.Redo](), policy.Slot.Inside(), func(c *policy.Ctx, _ vocab.Redo, _ policy.At[doc.SlotRef]) policy.Result {
		return policy.Result{History: &policy.HistoryRequest{Action: policy.HistoryRedo},
			EndComposition: c.Composition() != nil, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
	}).Any()
	p.Command("history.undoCorrection", "commands.undoCorrection", vocab.Undo{TopLayer: true})
})

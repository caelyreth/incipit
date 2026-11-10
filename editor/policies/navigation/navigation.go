// Package navigation is the navigation policy: Leave exits the innermost
// Leavable scope past its end, and Return enters the scope that ends
// right before the caret. Both read where the caret is, never how it got
// there: Return mirrors Leave, so after Leave it goes back in, and after a
// closer typed by hand it goes in all the same.
//
// TODO(navigation): declare the "which roles are leavable" option and match
// against it instead of hard-coding RoleLeavable.
package navigation

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Policy is the navigation policy.
var Policy = policy.Define("navigation", func(p *policy.P) {
	leavable := vocab.RoleScope(vocab.RoleLeavable)
	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).In(leavable).Do(leave)
	p.Handle("return").On(vocab.On(vocab.IntentReturn)).In(vocab.Slot).When(vocab.After(leavable)).Do(enter)
})

// leave exits the innermost leavable scope, past its end.
func leave(c *policy.Ctx, m policy.Match) policy.Result {
	l, ok := m.Role.(vocab.Leavable)
	if !ok {
		return policy.Swallow
	}
	res := policy.Select(doc.Collapsed(l.Exit()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// enter goes into the leavable scope that ends right before the caret,
// where Leave would have left it from: its entry.
func enter(c *policy.Ctx, _ policy.Match) policy.Result {
	data, ok := c.Beside(vocab.RoleLeavable, false)
	l, lok := data.(vocab.Leavable)
	if !ok || !lok {
		return policy.Swallow
	}
	res := policy.Select(doc.Collapsed(l.Entry()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

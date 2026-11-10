// Package navigation is the navigation policy: Leave exits the innermost
// configured Leavable scope past its end, and Return enters the scope
// that ends right before the caret. Both read where the caret is, never
// how it got there: Return mirrors Leave, so after Leave it goes back in,
// and after a closer typed by hand it goes in all the same.
package navigation

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "navigation"

// Roles chooses which role providers navigation treats as leavable.
var Roles = policy.List("roles", []vocab.Role{vocab.RoleLeavable}, []policy.Choice[vocab.Role]{
	{Value: vocab.RoleLeavable, Label: "settings.navigation.roles.leavable"},
	{Value: vocab.RolePair, Label: "settings.navigation.roles.pair"},
}, policy.Label("settings.navigation.roles"), policy.Description("settings.navigation.roles.description"))

// Policy is the navigation policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	roles := policy.GetList(p, Roles)
	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).In(vocab.Slot).Do(leave(roles))
	p.Handle("return").On(vocab.On(vocab.IntentReturn)).In(vocab.Slot).Do(enter(roles))
})

// leave exits the innermost leavable scope, past its end.
func leave(roles []vocab.Role) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		levels := c.Stack().Levels
		for i := len(levels) - 1; i >= 0; i-- {
			for _, role := range roles {
				data, ok := levels[i].Role(c.Doc(), role)
				l, lok := data.(vocab.Leavable)
				if !ok || !lok {
					continue
				}
				res := policy.Select(doc.Collapsed(l.Exit()))
				res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
				return res
			}
		}
		return policy.Swallow
	}
}

// enter goes into the leavable scope that ends right before the caret,
// where Leave would have left it from: its entry.
func enter(roles []vocab.Role) policy.Handler {
	return func(c *policy.Ctx, _ policy.Match) policy.Result {
		var found vocab.Leavable
		for _, role := range roles {
			data, ok := c.Beside(role, false)
			l, lok := data.(vocab.Leavable)
			if !ok || !lok {
				continue
			}
			if found == nil || doc.Compare(c.Doc(), l.Exit(), found.Exit()) > 0 {
				found = l
			}
		}
		if found == nil {
			return policy.Swallow
		}
		res := policy.Select(doc.Collapsed(found.Entry()))
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
}

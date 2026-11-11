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
	// Leave exits the innermost scope of any configured role; Return enters
	// one ending right before the caret, the roles in their order.
	for _, role := range policy.GetList(p, Roles) {
		scope := policy.Role[vocab.Leavable](role)
		policy.On(p, "leave-"+role.String(), policy.Intent[vocab.Leave](), scope.Inside(), leave).Any()
		policy.On(p, "return-"+role.String(), policy.Intent[vocab.Return](), scope.Behind(), enter).Any()
	}
})

// leave exits the scope, past its end.
func leave(_ *policy.Ctx, _ vocab.Leave, at policy.At[vocab.Leavable]) policy.Result {
	res := policy.Select(doc.Collapsed(at.Data.Exit()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// enter goes into the scope ending right before the caret, where Leave
// would have left it from: its entry.
func enter(_ *policy.Ctx, _ vocab.Return, at policy.At[vocab.Leavable]) policy.Result {
	res := policy.Select(doc.Collapsed(at.Data.Entry()))
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

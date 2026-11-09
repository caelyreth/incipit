// Package navigation is the navigation policy: Leave exits the innermost
// Leavable scope, and Return re-enters the scope just left.
//
// TODO(navigation): after the pointer moves far from the last edit, a
// time-limited return offer (a deadline annotation the guides policy
// shows); and the option of which roles are leavable once roles other
// than Leavable can be.
package navigation

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ReturnToEdit keeps the scope Leave exited, for Return.
var ReturnToEdit = policy.Bool("returnToEdit", true, policy.Label("settings.navigation.returnToEdit"))

// Policy is the navigation policy.
var Policy = policy.Define("navigation", func(p *policy.P) {
	remember := policy.Get(p, ReturnToEdit)
	p.Annotation(vocab.LeftScope, func(r doc.Reader, a policy.Annotation) bool {
		entry, ok := a.Data.(doc.Caret)
		if !ok {
			return false
		}
		_, exists := r.Node(entry.Pos.Node)
		return exists
	})
	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).In(vocab.RoleScope(vocab.RoleLeavable)).
		Do(func(c *policy.Ctx, m policy.Match) policy.Result {
			l, ok := m.Role.(vocab.Leavable)
			if !ok {
				return policy.Swallow
			}
			exit := l.Exit()
			res := policy.Select(doc.Collapsed(exit))
			res.DropKinds = []vocab.AnnotationKind{vocab.LeftScope}
			if remember {
				res.Annotate = []policy.Annotate{{Kind: vocab.LeftScope, Anchor: policy.PointAnchor(exit.Pos),
					Data: c.Head(), Lifetime: policy.UntilCaretLeaves(nil)}}
			}
			res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
			return res
		})
	if remember {
		p.Handle("return").On(vocab.On(vocab.IntentReturn)).When(vocab.Has(vocab.LeftScope)).
			Do(func(c *policy.Ctx, m policy.Match) policy.Result {
				res := policy.Result{Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
				for _, a := range c.Annotations().At(vocab.LeftScope, c.Head().Pos) {
					if entry, ok := a.Data.(doc.Caret); ok {
						s := doc.Collapsed(entry)
						res.Selection = &s
					}
					res.Drop = append(res.Drop, a.ID)
				}
				return res
			})
	}
})

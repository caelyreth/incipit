// Package navigation is the navigation policy: Leave exits the innermost
// Leavable scope, and Return re-enters the scope just left.
//
// TODO(navigation): the option of which roles are leavable once roles
// other than Leavable can be.
package navigation

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Policy is the navigation policy.
var Policy = policy.Define("navigation", func(p *policy.P) {
	exists := func(r doc.Reader, a policy.Annotation) bool {
		_, ok := r.Node(a.Anchor.From.Node)
		return ok
	}
	p.Annotation(vocab.LeftScope, exists)
	p.Handle("leave").On(vocab.On(vocab.IntentLeave)).In(vocab.RoleScope(vocab.RoleLeavable)).Do(leave)
	p.Handle("return").On(vocab.On(vocab.IntentReturn)).Do(back)
})

// leave exits the innermost leavable scope, remembering where the caret
// was for Return until the next edit or the next Leave: moving away and
// back to the exit finds the way back again; writing on moves on.
func leave(c *policy.Ctx, m policy.Match) policy.Result {
	l, ok := m.Role.(vocab.Leavable)
	if !ok {
		return policy.Swallow
	}
	exit := l.Exit()
	res := policy.Select(doc.Collapsed(exit))
	res.DropKinds = []vocab.AnnotationKind{vocab.LeftScope}
	res.Annotate = []policy.Annotate{{Kind: vocab.LeftScope, Anchor: policy.CaretAnchor(exit),
		Data: c.Head(), Lifetime: policy.UntilEdited}}
	res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
	return res
}

// back goes where the caret came from, into the scope just left. With
// none, it does nothing.
func back(c *policy.Ctx, _ policy.Match) policy.Result {
	for _, a := range c.Annotations().At(vocab.LeftScope, c.Head()) {
		to, ok := a.Data.(doc.Caret)
		if !ok {
			continue
		}
		res := policy.Select(doc.Collapsed(to))
		res.DropKinds = []vocab.AnnotationKind{vocab.LeftScope}
		res.Effects = []policy.Effect{{Kind: policy.EffectReveal}}
		return res
	}
	return policy.Swallow
}

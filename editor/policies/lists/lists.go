// Package lists is the lists policy, over items: Enter in an empty item
// steps it out of its list, and nesting moves an item into the item before
// it or one level out. Both step the item's own level (vocab.Leveled), as
// Backspace at an item's start does through core, so an item has one
// notion of its depth whichever key changes it.
package lists

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "lists"

// Policy is the lists policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Command("lists.bullet", "commands.bulletList", vocab.Reshape{Shape: profile.List})
	p.Command("lists.numbered", "commands.numberedList", vocab.Reshape{
		Shape: profile.List,
		Attrs: doc.MakeAttrs(doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
			doc.Field{Num: profile.FieldListStart, Value: doc.Int(1)}),
	})

	item := policy.NodeScope(profile.Item)
	p.Handle("leave-empty").On(policy.On(vocab.IntentBreak)).In(item).
		When(policy.Collapsed, policy.OwnText, policy.SlotEmpty).Produces(vocab.RoleEdits...).Do(leave)
	p.Handle("nest").On(policy.On(vocab.IntentNest)).In(item).When(policy.OwnText).Produces(vocab.RoleEdits...).Do(nest)
})

// level is the matched item's level.
func level(c *policy.Ctx, m policy.Match) (vocab.Leveled, bool) {
	if m.Level == nil || m.Level.Node == nil {
		return nil, false
	}
	role, ok := profile.NodeRole(c.Doc(), m.Level.Node, vocab.RoleLeveled)
	if !ok {
		return nil, false
	}
	l, ok := role.(vocab.Leveled)
	return l, ok
}

// leave steps an empty item one level out: a nested item follows the item
// holding it, a top-level one becomes a paragraph after its list.
func leave(c *policy.Ctx, m policy.Match) policy.Result {
	l, ok := level(c, m)
	if !ok {
		return policy.Swallow
	}
	return policy.Result{Edits: []vocab.Edit{l.Step(vocab.Backward)}, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// nest moves the item in (Forward) or out.
func nest(c *policy.Ctx, m policy.Match) policy.Result {
	in := m.Intent.(vocab.Nest)
	l, ok := level(c, m)
	if !ok {
		return policy.Swallow
	}
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Forward
	}
	return policy.Do(l.Step(dir))
}

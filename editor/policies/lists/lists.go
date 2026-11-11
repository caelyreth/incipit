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

	items := policy.Node(profile.Item)
	policy.On(p, "leave-empty", policy.Intent[vocab.Break](), items.Empty().Own(), leave)
	policy.On(p, "nest", policy.Intent[vocab.Nest](), items.Own(), nest).Any()
})

// level is the item's own level.
func level(c *policy.Ctx, item *doc.Node) (vocab.Leveled, bool) {
	role, ok := profile.NodeRole(c.Doc(), item, vocab.RoleLeveled)
	if !ok {
		return nil, false
	}
	l, ok := role.(vocab.Leveled)
	return l, ok
}

// leave steps an empty item one level out: a nested item follows the item
// holding it, a top-level one becomes a paragraph after its list.
func leave(c *policy.Ctx, _ vocab.Break, at policy.At[*doc.Node]) policy.Result {
	l, ok := level(c, at.Data)
	if !ok {
		return policy.Swallow
	}
	return policy.Result{Edits: []vocab.Edit{l.Step(vocab.Backward)}, Effects: []policy.Effect{{Kind: policy.EffectReveal}}}
}

// nest moves the item in (Forward) or out.
func nest(c *policy.Ctx, in vocab.Nest, at policy.At[*doc.Node]) policy.Result {
	l, ok := level(c, at.Data)
	if !ok {
		return policy.Swallow
	}
	dir := in.Dir
	if dir == 0 {
		dir = vocab.Forward
	}
	return policy.Do(l.Step(dir))
}

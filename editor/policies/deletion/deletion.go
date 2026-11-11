// Package deletion is the deletion policy: what is deleted as one is first
// shown as one. Deleting toward a whole, from beside it or from its inner
// edge, holds what it covers for the next delete, which removes it; so
// does a range selecting a node's content exactly. A whole is
// anything providing Structure, which says what it covers and how it goes:
// a node, a pair's two sides, a mark's signs.
package deletion

import (
	"slices"

	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "deletion"

// Roles chooses the roles whose providers are held before deletion.
var Roles = policy.List("roles", []vocab.Role{vocab.RoleStructure}, []policy.Choice[vocab.Role]{
	{Value: vocab.RoleStructure, Label: "settings.deletion.roles.structure"},
}, policy.Label("settings.deletion.roles"), policy.Description("settings.deletion.roles.description"))

// held is what the next delete takes: the first and last pieces of the
// whole's extent, which tell one whole from another.
type held struct{ first, last vocab.Edge }

func heldOf(extent []vocab.Edge) held { return held{extent[0], extent[len(extent)-1]} }

// Policy is the deletion policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	if !slices.Contains(policy.GetList(p, Roles), vocab.RoleStructure) {
		return
	}
	p.Annotation(policy.Armed, nil)
	p.Decorate(policy.Armed, vocab.GroupArmed)

	wholes := policy.Role[vocab.Structure](vocab.RoleStructure)
	// A whole's inner edges are left, never deleted across, when it is a
	// scope one leaves: deleting at them is deleting toward the whole.
	scopes := policy.Role[vocab.Leavable](vocab.RoleLeavable)
	back, fwd := policy.Deleting(0, vocab.Backward), policy.Deleting(0, vocab.Forward)
	policy.On(p, "behind", back, wholes.Behind(), toward)
	policy.On(p, "ahead", fwd, wholes.Ahead(), toward)
	policy.On(p, "at-start", back, policy.Or(scopes.AtStart(), scopes.Empty()), edge)
	policy.On(p, "at-end", fwd, policy.Or(scopes.AtEnd(), scopes.Empty()), edge)
	// A range over a node's content exactly holds the node. An object
	// selected as a node already shows itself whole: core deletes it at
	// once.
	policy.On(p, "selected", policy.Intent[vocab.Delete](), wholes.Selected(), selected).Range()
}, policy.Label("policies.deletion"), policy.Description("policies.deletion.description"))

func toward(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Structure]) policy.Result {
	return hold(c, at.Data.Extent(), at.Data.Remove())
}

// edge is a delete at a scope's inner edge: toward the scope, when it is a
// whole.
func edge(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Leavable]) policy.Result {
	role, ok := at.Role(c.Doc(), vocab.RoleStructure)
	if !ok {
		return policy.Pass
	}
	s := role.(vocab.Structure)
	return hold(c, s.Extent(), s.Remove())
}

// selected holds a node whose content a range selects exactly; the next
// delete removes it. A pair's or mark's content selected is text.
func selected(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Structure]) policy.Result {
	if at.Class != policy.LevelNode {
		return policy.Pass
	}
	return hold(c, at.Data.Extent(), at.Data.Remove())
}

// hold shows a whole for the next delete; held already, it is removed.
func hold(c *policy.Ctx, extent []vocab.Edge, remove vocab.Edit) policy.Result {
	if len(extent) == 0 {
		return policy.Swallow
	}
	res := policy.Result{DropKinds: []policy.AnnotationKind{policy.Armed}}
	key := heldOf(extent)
	if isHeld(c, key) {
		res.Edits = []vocab.Edit{remove}
		return res
	}
	for _, e := range extent {
		res.Annotate = append(res.Annotate, policy.Annotate{
			Kind: policy.Armed, Anchor: policy.Anchor{From: e.From, To: e.To}, Data: key, Lifetime: policy.UntilNextIntent,
		})
	}
	return res
}

func isHeld(c *policy.Ctx, key held) bool {
	for _, a := range c.Annotations().Of(policy.Armed) {
		if a.Owner == ID && a.Data == key {
			return true
		}
	}
	return false
}

// Package deletion is the deletion policy: what is deleted as one is first
// shown as one. Deleting toward a whole, from beside it or from its inner
// edge, holds what it covers for the next delete, which removes it. A whole is
// anything providing Structure, which says what it covers and how it goes:
// a node, a pair's two sides, a mark's signs.
package deletion

import (
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// ID is the policy's id.
const ID = "deletion"

// held is what the next delete takes: the first and last pieces of the
// whole's extent, which tell one whole from another.
type held struct{ first, last vocab.Edge }

func heldOf(extent []vocab.Edge) held { return held{extent[0], extent[len(extent)-1]} }

// Policy is the deletion policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Annotation(policy.Armed, nil)
	p.Decorate(policy.Armed, vocab.GroupArmed)
	wholes := policy.Role(vocab.RoleStructure)
	// A whole's inner edges are left, never deleted across, when it is a
	// scope one leaves: deleting out of it is deleting toward it.
	scopes := policy.Role(vocab.RoleLeavable)
	policy.On(p, "toward", policy.Intent[vocab.Delete](), policy.Or(wholes.Behind(), wholes.Ahead()), toward)
	policy.On(p, "out", policy.Intent[vocab.Delete](), policy.Or(scopes.StartEdge(), scopes.EndEdge()), out)
	// A range deletes the text it covers, however much of a whole's: a
	// whole is selected as a node, which shows itself whole, and core
	// deletes it at once.
}, policy.Label("policies.deletion"), policy.Description("policies.deletion.description"))

func toward(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Structure]) policy.Result {
	return hold(c, at.Data.Extent(), at.Data.Remove())
}

// out is a delete leaving a scope: toward the scope, when it is a whole.
func out(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Leavable]) policy.Result {
	s, ok := policy.LevelAs(c, at.Level, vocab.RoleStructure)
	if !ok {
		return policy.Pass
	}
	return hold(c, s.Extent(), s.Remove())
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

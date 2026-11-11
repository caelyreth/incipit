// Package deletion is the deletion policy: deleting into a structure
// arms the whole structure once through its Structure role, and the next
// delete removes it. Text pairs and mark spans keep the pairs policy's
// Pair-based arming.
package deletion

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "deletion"

// Roles chooses the roles whose providers arm before deletion.
var Roles = policy.List("roles", []vocab.Role{vocab.RoleStructure}, []policy.Choice[vocab.Role]{
	{Value: vocab.RoleStructure, Label: "settings.deletion.roles.structure"},
}, policy.Label("settings.deletion.roles"), policy.Description("settings.deletion.roles.description"))

// armed is the deletion policy's annotation data: the structure whole.
type armed struct{ Node doc.ID }

// Policy is the deletion policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	if !slices.Contains(policy.GetList(p, Roles), vocab.RoleStructure) {
		return
	}
	p.Annotation(policy.Armed, valid)
	p.Decorate(policy.Armed, vocab.GroupArmed)

	// A structure selected whole, as a node or as a text range over it
	// exactly, is deleted as a whole.
	policy.On(p, "structure", policy.Intent[vocab.Delete](), policy.Role[vocab.Structure](vocab.RoleStructure).Selected(), structure)
})

func structure(c *policy.Ctx, _ vocab.Delete, at policy.At[vocab.Structure]) policy.Result {
	return act(c, at.Node.ID())
}

// act arms the structure once and deletes it on the next delete.
func act(c *policy.Ctx, id doc.ID) policy.Result {
	n, ok := c.Doc().Node(id)
	if !ok {
		return policy.Swallow
	}
	res := policy.Result{}
	for _, a := range c.Annotations().Of(policy.Armed) {
		if a.Owner == ID {
			res.Drop = append(res.Drop, a.ID)
			if a.Data == (armed{Node: id}) {
				res.Edits = []vocab.Edit{vocab.RemoveNode(id)}
				return res
			}
		}
	}
	anchor, ok := structureAnchor(c.Doc(), n)
	if !ok {
		return policy.Swallow
	}
	res.Annotate = []policy.Annotate{{
		Kind: policy.Armed, Anchor: anchor, Data: armed{Node: id}, Lifetime: policy.UntilNextIntent,
	}}
	return res
}

// structureAnchor covers the structure whole. An inline node or a text
// block spans its slot's text; a container spans its place in the slot
// that holds it.
func structureAnchor(r doc.Reader, n *doc.Node) (policy.Anchor, bool) {
	parent, slot, index, ok := vocab.Index(r, n.ID())
	if !ok {
		return policy.Anchor{}, false
	}
	pn, _ := r.Node(parent)
	if pn.Slot(slot).IsInline() {
		off, ok := pn.Slot(slot).Content().NodeOffset(n.ID())
		if !ok {
			return policy.Anchor{}, false
		}
		return policy.SpanAnchor(vocab.Span{Slot: doc.SlotRef{Node: parent, Slot: slot}, From: off, To: off + 1}), true
	}
	for i, s := range n.Slots() {
		if s.IsInline() {
			return policy.SpanAnchor(vocab.Span{Slot: doc.SlotRef{Node: n.ID(), Slot: i}, From: 0, To: s.Len()}), true
		}
	}
	at := doc.Pos{Node: parent, Slot: slot, Offset: index}
	return policy.Between(at, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}), true
}

// valid drops an arm once its structure is gone; act re-checks the role
// before deleting. An armed annotation is the deletion policy's own;
// pairs' arms have no armed data and are checked by the pairs policy.
func valid(r doc.Reader, a policy.Annotation) bool {
	data, ok := a.Data.(armed)
	if !ok {
		return false
	}
	_, ok = r.Node(data.Node)
	return ok
}

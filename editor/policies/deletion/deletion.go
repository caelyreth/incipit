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
	p.Annotation(vocab.Armed, valid)
	p.Decorate(vocab.Armed, vocab.GroupArmed)

	// A text range exactly over a structure is the structure, deleted as
	// a whole. A range is not collapsed, so these signatures cannot meet
	// the node-selected rule in this policy.
	p.Handle("structure").On(vocab.On(vocab.IntentDelete)).In(vocab.RoleScope(vocab.RoleStructure)).
		When(vocab.TextRange, vocab.Equals).Unless(vocab.Collapsed).Produces(vocab.RoleEdits...).Do(structure)

	// An object selected whole has no node level of its own in the scope
	// stack: its head is the gap before it. The page level sees every
	// node selection and checks the role by id.
	p.Handle("selected").On(vocab.On(vocab.IntentDelete)).In(vocab.NodeScope(vocab.Page)).
		When(vocab.NodeSelected(0)).Produces(vocab.RoleEdits...).Do(selected)
})

func structure(c *policy.Ctx, m policy.Match) policy.Result {
	if m.Level == nil || m.Level.Node == nil {
		return policy.Swallow
	}
	return act(c, m.Level.Node.ID())
}

func selected(c *policy.Ctx, _ policy.Match) policy.Result {
	return act(c, c.Selection().Node)
}

// act arms the structure once and deletes it on the next delete.
func act(c *policy.Ctx, id doc.ID) policy.Result {
	n, ok := c.Doc().Node(id)
	if !ok || !slices.Contains(vocab.NodeRoles(n.Kind()), vocab.RoleStructure) {
		return policy.Swallow
	}
	res := policy.Result{}
	for _, a := range c.Annotations().Of(vocab.Armed) {
		if a.Owner == ID {
			res.Drop = append(res.Drop, a.ID)
			if a.Data == (armed{Node: id}) {
				res.Edits = []vocab.Edit{remove(id)}
				return res
			}
		}
	}
	anchor, ok := structureAnchor(c.Doc(), n)
	if !ok {
		return policy.Swallow
	}
	res.Annotate = []policy.Annotate{{
		Kind: vocab.Armed, Anchor: anchor, Data: armed{Node: id}, Lifetime: policy.UntilNextIntent,
	}}
	return res
}

// remove takes a block out of its slot, or an inline structure out of
// its slot's items.
func remove(id doc.ID) vocab.Edit {
	return func(tx *doc.Tx) error {
		parent, slot, ok := tx.Parent(id)
		if !ok {
			return doc.ErrNoNode
		}
		pn, _ := tx.Node(parent)
		if !pn.Slot(slot).IsInline() {
			if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
				return err
			}
			// The page keeps a block to type in when its last structure
			// goes. Other slots keep their own empty form.
			pn, _ = tx.Node(parent)
			if pn.Kind() == vocab.Page && pn.Slot(slot).Len() == 0 {
				return tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: 0, Nodes: []*doc.Node{vocab.NewParagraph()}})
			}
			return nil
		}
		off, ok := pn.Slot(slot).Content().NodeOffset(id)
		if !ok {
			return doc.ErrNoNode
		}
		return tx.Apply(doc.EditText{
			At: doc.SlotRef{Node: parent, Slot: slot}, From: off, To: off + 1,
		})
	}
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

// valid drops an arm once its structure is gone or no longer a
// structure. An armed annotation is the deletion policy's own; pairs'
// arms have no armed data and are checked by the pairs policy.
func valid(r doc.Reader, a policy.Annotation) bool {
	data, ok := a.Data.(armed)
	if !ok {
		return false
	}
	n, ok := r.Node(data.Node)
	return ok && slices.Contains(vocab.NodeRoles(n.Kind()), vocab.RoleStructure)
}

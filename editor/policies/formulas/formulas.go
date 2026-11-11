// Package formulas is the formulas policy: "$" typed where an inline
// formula may stand makes one with the caret in its source, and "$" in an
// empty formula alone in its paragraph makes a math block. A formula's
// spelling closes it: "$" at its source's end steps past it, as navigation
// steps past every Closer.
package formulas

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "formulas"

// Policy is the formulas policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	dollar := policy.Typed(string(profile.FormulaSpelling))
	policy.On(p, "open", dollar, policy.Slot.Own(), open).When(policy.Takes(profile.Formula))
	policy.On(p, "math", dollar, policy.Node(profile.Formula).Empty(), toMath)
})

// open inserts an empty formula and puts the caret in its source.
func open(c *policy.Ctx, _ vocab.Type, _ policy.At[doc.SlotRef]) policy.Result {
	head := c.Head().Pos
	formula := vocab.NewBlock(c.Doc().Schema(), profile.Formula, doc.Attrs{})
	res := policy.Do(vocab.Ops(doc.EditText{At: head.Ref(), From: head.Offset, To: head.Offset, Items: []doc.Item{doc.NodeItem(formula)}}))
	res.Then = func(r doc.Reader) doc.Selection {
		n, _ := r.Node(head.Node)
		if it, _, ok := n.Slot(head.Slot).Content().ItemAt(head.Offset); ok && it.Node != nil {
			return doc.Collapsed(doc.CaretAt(doc.Pos{Node: it.Node.ID()}))
		}
		return doc.Collapsed(doc.CaretAt(head))
	}
	return res
}

// toMath: "$" in an empty formula alone in its paragraph makes the
// paragraph a math block; elsewhere the "$" is the formula's source, not
// its close.
func toMath(c *policy.Ctx, _ vocab.Type, at policy.At[*doc.Node]) policy.Result {
	r := c.Doc()
	id := at.Data.ID()
	parent, slot, ok := r.Parent(id)
	pn, _ := r.Node(parent)
	if !ok || pn.Kind() != profile.Paragraph || pn.Slot(slot).Len() != 1 {
		return policy.Do(vocab.Ops(doc.EditText{At: doc.SlotRef{Node: id}, Items: []doc.Item{doc.TextItem(string(profile.FormulaSpelling), nil)}}))
	}
	gp, gslot, index, _ := vocab.Index(r, parent)
	res := policy.Do(func(tx *doc.Tx) error {
		if err := tx.Apply(doc.Remove{IDs: []doc.ID{parent}}); err != nil {
			return err
		}
		return tx.Apply(doc.Insert{Parent: gp, Slot: gslot, Index: index, Nodes: []*doc.Node{vocab.NewBlock(tx.Schema(), profile.Math, doc.Attrs{})}})
	})
	res.Then = func(r doc.Reader) doc.Selection {
		c, _ := vocab.BlockCaret(r, doc.Pos{Node: gp, Slot: gslot, Offset: index}, vocab.Backward)
		return doc.Collapsed(c)
	}
	return res
}

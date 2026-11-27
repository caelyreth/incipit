package vocab

import (
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/incipit/internal/editor/doc"
)

// --- blocks -----------------------------------------------------------

// SetKind gives a node another kind, with attrs.
func SetKind(id doc.ID, kind doc.Kind, attrs doc.Attrs) Edit {
	return Ops(doc.SetNode{ID: id, Kind: kind, Attrs: attrs})
}

// SetAttrs changes a node's attributes, keeping its kind.
func SetAttrs(id doc.ID, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		return tx.Apply(doc.SetNode{ID: id, Kind: n.Kind(), Attrs: attrs})
	}
}

// Split splits the block holding at in two: a new node of kind follows it,
// and the text after at moves into the new node's first text slot.
// Everything after at in the block's later slots stays.
func Split(at doc.Pos, kind doc.Kind, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(at.Node)
		if !ok || at.Slot >= n.NumSlots() || !n.Slot(at.Slot).IsInline() {
			return fmt.Errorf("%w: split outside a text slot", ErrHelper)
		}
		parent, slot, index, ok := Index(tx, at.Node)
		if !ok {
			return fmt.Errorf("%w: split of the page", ErrHelper)
		}
		pn, _ := tx.Node(parent)
		if pn.Slot(slot).IsInline() {
			return fmt.Errorf("%w: split of an inline node", ErrHelper)
		}
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: []*doc.Node{NewBlock(tx.Schema(), kind, attrs)}}); err != nil {
			return err
		}
		pn, _ = tx.Node(parent)
		created := pn.Slot(slot).Child(index + 1)
		target, ok := FirstTextSlot(created)
		if !ok {
			return nil
		}
		end := n.Slot(at.Slot).Len()
		if at.Offset == end {
			return nil
		}
		return tx.Apply(doc.Transfer{From: at.Ref(), Start: at.Offset, End: end, To: target, At: 0})
	}
}

// CanJoin reports whether second's text can move onto the end of first's:
// both are text slots of the same literalness, and line breaks are kept
// where second has them. An object is never dissolved into what is before
// it, and its own slot takes nothing from beside it; but a second node
// holding nothing moves nothing, so it joins any text slot, going.
func CanJoin(r doc.Reader, first, second doc.SlotRef) bool {
	a, ok1 := SlotSpec(r, first)
	b, ok2 := SlotSpec(r, second)
	if !ok1 || !ok2 || a.Blocks || b.Blocks {
		return false
	}
	sn, ok := r.Node(second.Node)
	if !ok || inObject(r, second.Node) {
		return false
	}
	if HoldsNothing(sn) {
		return true
	}
	if inObject(r, first.Node) || a.Literal != b.Literal {
		return false
	}
	if !a.Lines {
		n, _ := r.Node(second.Node)
		if strings.ContainsRune(n.Slot(second.Slot).Content().Text(), '\n') {
			return false
		}
	}
	return true
}

// inObject reports whether node id is an object or lies inside one. An
// object's text is its own: it never takes another block's text by
// joining, and what it holds never leaves it so.
func inObject(r doc.Reader, id doc.ID) bool {
	_, ok := ObjectOf(r, id)
	return ok
}

// ObjectOf is the object whose text node id's text is: the nearest object
// holding id, id itself included. An object is one unit whose text,
// however deep, is its surfaces, as a table's is in its cells; the nodes
// between it and its text only arrange them.
func ObjectOf(r doc.Reader, id doc.ID) (doc.ID, bool) {
	for n, ok := r.Node(id); ok; {
		if r.Schema().IsObject(n.Kind()) {
			return n.ID(), true
		}
		parent, _, has := r.Parent(n.ID())
		if !has {
			break
		}
		n, ok = r.Node(parent)
	}
	return 0, false
}

// CanLift reports whether a block lifted stays what it is: the slot
// holding id's container takes id's kind. Lift itself also moves what is
// converted after, as an item leaving its list.
func CanLift(r doc.Reader, id doc.ID) bool {
	n, ok := r.Node(id)
	if !ok {
		return false
	}
	container, _, ok := r.Parent(id)
	return ok && Accepts(r, container, n.Kind())
}

// HoldsNothing reports whether every slot of n is empty.
func HoldsNothing(n *doc.Node) bool {
	for _, s := range n.Slots() {
		if s.Len() > 0 {
			return false
		}
	}
	return true
}

// Join moves second's text onto the end of first's text slot, moves
// second's block children after first (into first's body when it has
// one), and removes second, and then containers it leaves empty, as a
// list whose only item joins the block before it.
func Join(first doc.SlotRef, second doc.ID) Edit {
	return func(tx *doc.Tx) error {
		sn, ok := tx.Node(second)
		if !ok {
			return doc.ErrNoNode
		}
		container, _, _ := tx.Parent(second)
		fn, ok := tx.Node(first.Node)
		if !ok {
			return doc.ErrNoNode
		}
		src, ok := FirstTextSlot(sn)
		if ok && src.Node == second {
			if !CanJoin(tx, first, src) {
				return fmt.Errorf("%w: incompatible slots never merge", ErrHelper)
			}
			// second's text is first's from here, its places with it.
			l, at := sn.Slot(src.Slot).Len(), fn.Slot(first.Slot).Len()
			if err := tx.Apply(doc.Transfer{From: src, Start: 0, End: l, To: first, At: at, Whole: true}); err != nil {
				return err
			}
		}
		sn, _ = tx.Node(second)
		if err := moveChildrenAfter(tx, sn, first.Node); err != nil {
			return err
		}
		if err := tx.Apply(doc.Remove{IDs: []doc.ID{second}}); err != nil {
			return err
		}
		return RemoveIfEmpty(container)(tx)
	}
}

// moveChildrenAfter moves the block children of from into target's body
// slot, or after target when it has none.
func moveChildrenAfter(tx *doc.Tx, from *doc.Node, target doc.ID) error {
	var moving []doc.ID
	for _, s := range from.Slots() {
		if s.IsInline() {
			continue
		}
		for _, c := range s.Children() {
			moving = append(moving, c.ID())
		}
	}
	if len(moving) == 0 {
		return nil
	}
	tn, _ := tx.Node(target)
	body := -1
	for i, s := range tn.Slots() {
		if !s.IsInline() {
			body = i
		}
	}
	for _, id := range moving {
		var err error
		if body >= 0 {
			tn, _ = tx.Node(target)
			err = tx.Apply(doc.Move{ID: id, Parent: target, Slot: body, Index: tn.Slot(body).Len()})
		} else {
			parent, slot, index, _ := Index(tx, target)
			err = tx.Apply(doc.Move{ID: id, Parent: parent, Slot: slot, Index: index + 1})
			target = id
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Wrap puts contiguous siblings into a new container of kind.
func Wrap(ids []doc.ID, kind doc.Kind, attrs doc.Attrs) Edit {
	return func(tx *doc.Tx) error {
		if len(ids) == 0 {
			return nil
		}
		parent, slot, index, ok := Index(tx, ids[0])
		if !ok {
			return doc.ErrNoNode
		}
		if err := tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: []*doc.Node{NewBlock(tx.Schema(), kind, attrs)}}); err != nil {
			return err
		}
		pn, _ := tx.Node(parent)
		container := pn.Slot(slot).Child(index).ID()
		body := bodySlot(tx.Schema(), kind)
		for i, id := range ids {
			if err := tx.Apply(doc.Move{ID: id, Parent: container, Slot: body, Index: i}); err != nil {
				return err
			}
		}
		return nil
	}
}

// bodySlot is the block slot of a container kind.
func bodySlot(s *doc.Schema, kind doc.Kind) int {
	spec, _ := s.Kind(kind)
	for i, s := range spec.Slots {
		if s.Blocks {
			return i
		}
	}
	return 0
}

// Unwrap moves a container's children out in its place and removes it.
func Unwrap(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		parent, slot, index, ok := Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		var moving []doc.ID
		for _, s := range n.Slots() {
			if !s.IsInline() {
				for _, c := range s.Children() {
					moving = append(moving, c.ID())
				}
			}
		}
		for i, c := range moving {
			if err := tx.Apply(doc.Move{ID: c, Parent: parent, Slot: slot, Index: index + i}); err != nil {
				return err
			}
		}
		return tx.Apply(doc.Remove{IDs: []doc.ID{id}})
	}
}

// Lift moves a node out of its container, to just after it. A container
// left empty is removed.
func Lift(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		container, _, _, ok := Index(tx, id)
		if !ok {
			return doc.ErrNoNode
		}
		cn, _ := tx.Node(container)
		grand, cslot, cindex, ok := Index(tx, container)
		if !ok {
			return fmt.Errorf("%w: lifting out of the page", ErrHelper)
		}
		// Siblings after id stay in the container, which splits around it:
		// they move into a copy of the container after the lifted node.
		parent, slot, index, _ := Index(tx, id)
		pn, _ := tx.Node(parent)
		// Moves edit a slot the transaction owns in place: what moves is
		// read before.
		rest := slices.Clone(pn.Slot(slot).Children()[index+1:])
		if err := tx.Apply(doc.Move{ID: id, Parent: grand, Slot: cslot, Index: cindex + 1}); err != nil {
			return err
		}
		if len(rest) > 0 {
			tail := NewBlock(tx.Schema(), cn.Kind(), cn.Attrs())
			if err := tx.Apply(doc.Insert{Parent: grand, Slot: cslot, Index: cindex + 2, Nodes: []*doc.Node{tail}}); err != nil {
				return err
			}
			gn, _ := tx.Node(grand)
			tailID := gn.Slot(cslot).Child(cindex + 2).ID()
			for i, r := range rest {
				if err := tx.Apply(doc.Move{ID: r.ID(), Parent: tailID, Slot: slot, Index: i}); err != nil {
					return err
				}
			}
		}
		return RemoveIfEmpty(container)(tx)
	}
}

// Sink moves an item into the body of the item before it, into the list
// that body ends with, or a new list of its own list's style.
func Sink(item doc.ID) Edit {
	return func(tx *doc.Tx) error {
		list, slot, index, ok := Index(tx, item)
		if !ok || index == 0 {
			return fmt.Errorf("%w: no item before to sink into", ErrHelper)
		}
		ln, _ := tx.Node(list)
		prev := ln.Slot(slot).Child(index - 1)
		body := prev.Slot(1)
		target := doc.ID(0)
		if n := body.Len(); n > 0 && body.Child(n-1).Kind() == ln.Kind() {
			target = body.Child(n - 1).ID()
		} else {
			if err := tx.Apply(doc.Insert{Parent: prev.ID(), Slot: 1, Index: n, Nodes: []*doc.Node{NewBlock(tx.Schema(), ln.Kind(), ln.Attrs())}}); err != nil {
				return err
			}
			pn, _ := tx.Node(prev.ID())
			target = pn.Slot(1).Child(n).ID()
		}
		tn, _ := tx.Node(target)
		return tx.Apply(doc.Move{ID: item, Parent: target, Slot: 0, Index: tn.Slot(0).Len()})
	}
}

// Outdent moves an item one level out: from a nested list to just after
// the item holding that list, the items after it in its list moving into
// its body to keep their order; an item of a top-level list leaves it as a
// paragraph (Unlist).
func Outdent(item doc.ID) Edit {
	return func(tx *doc.Tx) error {
		list, slot, index, ok := Index(tx, item)
		if !ok {
			return doc.ErrNoNode
		}
		owner, _, _, ok := Index(tx, list)
		if !ok {
			return doc.ErrNoNode
		}
		if on, _ := tx.Node(owner); !tx.Schema().IsEntry(on.Kind()) {
			return Unlist(item)(tx)
		}
		ln, _ := tx.Node(list)
		rest := slices.Clone(ln.Slot(slot).Children()[index+1:])
		outer, oslot, oindex, _ := Index(tx, owner)
		if err := tx.Apply(doc.Move{ID: item, Parent: outer, Slot: oslot, Index: oindex + 1}); err != nil {
			return err
		}
		if len(rest) > 0 {
			n, _ := tx.Node(item)
			at := n.Slot(1).Len()
			if err := tx.Apply(doc.Insert{Parent: item, Slot: 1, Index: at, Nodes: []*doc.Node{NewBlock(tx.Schema(), ln.Kind(), ln.Attrs())}}); err != nil {
				return err
			}
			n, _ = tx.Node(item)
			tail := n.Slot(1).Child(at).ID()
			for i, r := range rest {
				if err := tx.Apply(doc.Move{ID: r.ID(), Parent: tail, Slot: 0, Index: i}); err != nil {
					return err
				}
			}
		}
		return RemoveIfEmpty(list)(tx)
	}
}

// MoveTo moves a node to a block gap.
func MoveTo(id doc.ID, target doc.Pos) Edit {
	return Ops(doc.Move{ID: id, Parent: target.Node, Slot: target.Slot, Index: target.Offset})
}

// RemoveIfEmpty removes a container whose block slots hold nothing, and
// then its container if that is left empty too. The page is never
// removed, and a container with text is not empty.
func RemoveIfEmpty(id doc.ID) Edit {
	return func(tx *doc.Tx) error {
		for {
			n, ok := tx.Node(id)
			if !ok {
				return nil
			}
			parent, _, ok := tx.Parent(id)
			if !ok || !emptyContainer(n) {
				return nil
			}
			if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
				return err
			}
			id = parent
		}
	}
}

func emptyContainer(n *doc.Node) bool {
	blocks := false
	for _, s := range n.Slots() {
		if s.IsInline() {
			return false
		}
		blocks = true
		if s.Len() > 0 {
			return false
		}
	}
	return blocks
}

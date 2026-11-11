package vocab

import "github.com/caelyreth/neverland/editor/doc"

// Text slots are the inline slots of block nodes, which the caret walks in
// reading order. The slots of inline nodes, such as a formula's source,
// are entered only through their own stops.

// NewBlock builds an empty node of kind with the slots its kind declares.
func NewBlock(s *doc.Schema, kind doc.Kind, attrs doc.Attrs) *doc.Node {
	spec, _ := s.Kind(kind)
	slots := make([]doc.Slot, len(spec.Slots))
	for i, s := range spec.Slots {
		if s.Blocks {
			slots[i] = doc.BlockSlot()
		} else {
			slots[i] = doc.InlineSlot()
		}
	}
	return doc.NewNode(kind, attrs, slots...)
}

// FirstTextSlot is the first text slot at or below n.
func FirstTextSlot(n *doc.Node) (doc.SlotRef, bool) {
	for i, s := range n.Slots() {
		if s.IsInline() {
			return doc.SlotRef{Node: n.ID(), Slot: i}, true
		}
		for _, child := range s.Children() {
			if ref, ok := FirstTextSlot(child); ok {
				return ref, true
			}
		}
	}
	return doc.SlotRef{}, false
}

// LastTextSlot is the last text slot at or below n.
func LastTextSlot(n *doc.Node) (doc.SlotRef, bool) {
	slots := n.Slots()
	for i := len(slots) - 1; i >= 0; i-- {
		s := slots[i]
		if s.IsInline() {
			return doc.SlotRef{Node: n.ID(), Slot: i}, true
		}
		children := s.Children()
		for j := len(children) - 1; j >= 0; j-- {
			if ref, ok := LastTextSlot(children[j]); ok {
				return ref, true
			}
		}
	}
	return doc.SlotRef{}, false
}

// NextTextSlot is the text slot after ref in reading order.
func NextTextSlot(r doc.Reader, ref doc.SlotRef) (doc.SlotRef, bool) {
	n, ok := r.Node(ref.Node)
	if !ok {
		return doc.SlotRef{}, false
	}
	// Later slots of the same node first: an item's body after its head.
	for i := ref.Slot + 1; i < n.NumSlots(); i++ {
		if next, ok := firstIn(n, i); ok {
			return next, true
		}
	}
	return after(r, n)
}

func firstIn(n *doc.Node, slot int) (doc.SlotRef, bool) {
	s := n.Slot(slot)
	if s.IsInline() {
		return doc.SlotRef{Node: n.ID(), Slot: slot}, true
	}
	for _, child := range s.Children() {
		if ref, ok := FirstTextSlot(child); ok {
			return ref, true
		}
	}
	return doc.SlotRef{}, false
}

func lastIn(n *doc.Node, slot int) (doc.SlotRef, bool) {
	s := n.Slot(slot)
	if s.IsInline() {
		return doc.SlotRef{Node: n.ID(), Slot: slot}, true
	}
	children := s.Children()
	for j := len(children) - 1; j >= 0; j-- {
		if ref, ok := LastTextSlot(children[j]); ok {
			return ref, true
		}
	}
	return doc.SlotRef{}, false
}

// after is the first text slot after n's subtree.
func after(r doc.Reader, n *doc.Node) (doc.SlotRef, bool) {
	for {
		parent, slot, ok := r.Parent(n.ID())
		if !ok {
			return doc.SlotRef{}, false
		}
		pn, _ := r.Node(parent)
		s := pn.Slot(slot)
		if s.IsInline() {
			// n is an inline node; its parent's slot continues.
			n = pn
			continue
		}
		i, _ := s.IndexOf(n)
		for _, sibling := range s.Children()[i+1:] {
			if ref, ok := FirstTextSlot(sibling); ok {
				return ref, true
			}
		}
		for j := slot + 1; j < pn.NumSlots(); j++ {
			if ref, ok := firstIn(pn, j); ok {
				return ref, true
			}
		}
		n = pn
	}
}

// PrevTextSlot is the text slot before ref in reading order.
func PrevTextSlot(r doc.Reader, ref doc.SlotRef) (doc.SlotRef, bool) {
	n, ok := r.Node(ref.Node)
	if !ok {
		return doc.SlotRef{}, false
	}
	for i := ref.Slot - 1; i >= 0; i-- {
		if prev, ok := lastIn(n, i); ok {
			return prev, true
		}
	}
	for {
		parent, slot, ok := r.Parent(n.ID())
		if !ok {
			return doc.SlotRef{}, false
		}
		pn, _ := r.Node(parent)
		s := pn.Slot(slot)
		if s.IsInline() {
			n = pn
			continue
		}
		i, _ := s.IndexOf(n)
		children := s.Children()
		for j := i - 1; j >= 0; j-- {
			if ref, ok := LastTextSlot(children[j]); ok {
				return ref, true
			}
		}
		for j := slot - 1; j >= 0; j-- {
			if ref, ok := lastIn(pn, j); ok {
				return ref, true
			}
		}
		n = pn
	}
}

// Block returns the block node holding p: p's node, or for a position in
// an inline node's slot, the block that holds the inline node.
func Block(r doc.Reader, p doc.Pos) (*doc.Node, bool) {
	id := p.Node
	for {
		n, ok := r.Node(id)
		if !ok {
			return nil, false
		}
		parent, slot, ok := r.Parent(id)
		if !ok {
			return n, true
		}
		pn, _ := r.Node(parent)
		if !pn.Slot(slot).IsInline() {
			return n, true
		}
		id = parent
	}
}

// SlotSpec returns the declaration of a slot.
func SlotSpec(r doc.Reader, ref doc.SlotRef) (doc.SlotSpec, bool) {
	n, ok := r.Node(ref.Node)
	if !ok {
		return doc.SlotSpec{}, false
	}
	spec, ok := r.Schema().Kind(n.Kind())
	if !ok || ref.Slot >= len(spec.Slots) {
		return doc.SlotSpec{}, false
	}
	return spec.Slots[ref.Slot], true
}

// Index returns where id stands among its siblings.
func Index(r doc.Reader, id doc.ID) (parent doc.ID, slot, index int, ok bool) {
	parent, slot, ok = r.Parent(id)
	if !ok {
		return 0, 0, 0, false
	}
	pn, _ := r.Node(parent)
	n, _ := r.Node(id)
	if pn.Slot(slot).IsInline() {
		index, ok = pn.Slot(slot).Content().NodeOffset(id)
		return parent, slot, index, ok
	}
	index, ok = pn.Slot(slot).IndexOf(n)
	return parent, slot, index, ok
}

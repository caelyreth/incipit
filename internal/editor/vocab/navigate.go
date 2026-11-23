package vocab

import (
	"slices"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

// Text slots are the inline slots of block nodes, which the caret walks in
// reading order. The slots of inline nodes, such as a formula's source,
// are entered only through their own stops.

// NewBlock builds a node of kind with the slots its kind declares, empty
// but for text, which fills its first inline slot.
func NewBlock(s *doc.Schema, kind doc.Kind, attrs doc.Attrs, text ...doc.Item) *doc.Node {
	spec, _ := s.Kind(kind)
	slots := make([]doc.Slot, len(spec.Slots))
	for i, s := range spec.Slots {
		if s.Blocks {
			slots[i] = doc.BlockSlot()
		} else {
			slots[i] = doc.InlineSlot(text...)
			text = nil
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

// Inline reports whether id stands in an inline slot: an inline node in
// its line.
func Inline(r doc.Reader, id doc.ID) bool {
	parent, slot, ok := r.Parent(id)
	if !ok {
		return false
	}
	pn, _ := r.Node(parent)
	return pn.Slot(slot).IsInline()
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

// Beside is the place right before a node (Backward) or right after it
// (Forward) in the slot that holds it: a gap for a block, an offset in
// its line for an inline node.
func Beside(r doc.Reader, id doc.ID, dir Direction) (doc.Pos, bool) {
	parent, slot, index, ok := Index(r, id)
	if !ok {
		return doc.Pos{}, false
	}
	if dir > 0 {
		index++
	}
	return doc.Pos{Node: parent, Slot: slot, Offset: index}, true
}

// NodeEdge is a node's place in the slot that holds it, from right
// before it to right after it.
func NodeEdge(r doc.Reader, id doc.ID) (Edge, bool) {
	before, ok := Beside(r, id, Backward)
	if !ok {
		return Edge{}, false
	}
	after, _ := Beside(r, id, Forward)
	return Edge{From: doc.CaretAt(before), To: doc.CaretAt(after)}, true
}

// ShapeSelection keeps a range's ends in one object's text or outside every
// object, as an object's text is its own: an end inside an object the
// other end is not in goes out to that object's edge, so the range takes
// it whole; and a range between two text slots of one object that is
// Spanned, as a table's cells are, becomes the selection of its shape.
// Every other selection is as it is.
func ShapeSelection(r doc.Reader, p Profile, s doc.Selection) doc.Selection {
	if s.Kind != doc.SelectText || s.IsCollapsed() {
		return s
	}
	first := doc.Compare(r, s.Anchor, s.Head) <= 0
	a, h := objectsAround(r, s.Anchor.Pos), objectsAround(r, s.Head.Pos)
	s.Anchor = outOf(r, s.Anchor, a, h, first)
	s.Head = outOf(r, s.Head, h, a, !first)
	common := objectsAround(r, s.Anchor.Pos)
	if len(common) == 0 || s.Anchor.Pos.Ref() == s.Head.Pos.Ref() {
		return s
	}
	n, _ := r.Node(common[0])
	if sp, ok := p.Role(r, n, RoleSpanned.ID); ok {
		if out, ok := sp.(Spanned).Span(s.Anchor, s.Head); ok {
			return out
		}
	}
	return s
}

// objectsAround are the objects holding p, the innermost first.
func objectsAround(r doc.Reader, p doc.Pos) []doc.ID {
	var out []doc.ID
	for _, id := range append([]doc.ID{p.Node}, doc.Ancestors(r, p.Node)...) {
		if n, ok := r.Node(id); ok && r.Schema().IsObject(n.Kind()) {
			out = append(out, id)
		}
	}
	return out
}

// outOf moves an end out of the outermost object holding it that does not
// hold the other end: before it when it is the range's first end, after
// it when its last.
func outOf(r doc.Reader, end doc.Caret, mine, theirs []doc.ID, first bool) doc.Caret {
	for i := len(mine) - 1; i >= 0; i-- {
		if slices.Contains(theirs, mine[i]) {
			continue
		}
		dir := Forward
		if first {
			dir = Backward
		}
		if at, ok := Beside(r, mine[i], dir); ok {
			return doc.CaretAt(at)
		}
	}
	return end
}

// TextEdge is a block's text, from the first stop at its start to the last
// at its end: the block's first text slot whole.
func TextEdge(n *doc.Node) (Edge, bool) {
	ref, ok := FirstTextSlot(n)
	if !ok {
		return Edge{}, false
	}
	return Edge{
		From: doc.Caret{Pos: doc.At(ref, 0), Stop: doc.StopFirst},
		To:   doc.Caret{Pos: doc.At(ref, n.Slot(ref.Slot).Len()), Stop: doc.StopLast},
	}, true
}

// SelectionExtent is what a selection spans, as pieces in reading order,
// as a whole's Structure extent is: a range between its ends, a selected
// node's place, or the text of each cell of a cell range, so what is
// shown selected is what an edit of it takes, and a cell emptied stays.
// A caret spans nothing.
func SelectionExtent(r doc.Reader, sel doc.Selection) []Edge {
	switch sel.Kind {
	case doc.SelectText:
		if !sel.IsCollapsed() {
			from, to := sel.Ordered(r)
			return []Edge{{From: from, To: to}}
		}
	case doc.SelectNode:
		if e, ok := NodeEdge(r, sel.Node); ok {
			return []Edge{e}
		}
	case doc.SelectCells:
		var out []Edge
		for _, c := range Cells(r, sel.Cells) {
			if e, ok := TextEdge(c); ok {
				out = append(out, e)
			}
		}
		return out
	}
	return nil
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

package doc

import (
	"slices"
	"strings"
)

// Slot is a node's typed content area: ordered block children, or inline
// content. The node's kind says which.
//
// TODO(memory): a block slot above a size threshold should be a persistent
// chunked vector with fanout 32 (§6), so replacing one child of a page with
// thousands copies a leaf and a path, not the whole slice.
type Slot struct {
	inline   bool
	children []*Node
	content  *Inline
	// owned marks a children slice private to a transaction, whose
	// children may be replaced in place, each where it stands. A published
	// slot is never owned.
	owned bool
}

// splice replaces del children from i with ins, in a new slice: a slice
// once read from a slot never shifts under whoever holds it, even within
// the transaction changing the slot, so a loop over a slot's children may
// move them out of it.
func (s *Slot) splice(i, del int, ins ...*Node) {
	s.children = slices.Concat(s.children[:i:i], ins, s.children[i+del:])
	s.owned = true
}

// BlockSlot holds block children.
func BlockSlot(children ...*Node) Slot {
	return Slot{children: slices.Clone(children)}
}

// InlineSlot holds inline items.
func InlineSlot(items ...Item) Slot {
	return Slot{inline: true, content: NewInline(items...)}
}

// IsInline reports whether the slot holds inline content.
func (s Slot) IsInline() bool { return s.inline }

// Children returns a block slot's children. The slice must not change, and
// no edit shifts it: one inserting or removing children makes a new one.
func (s Slot) Children() []*Node { return s.children }

// Child returns a block slot's i-th child.
func (s Slot) Child(i int) *Node { return s.children[i] }

// Content returns an inline slot's content.
func (s Slot) Content() *Inline {
	if s.content == nil {
		return emptyInline
	}
	return s.content
}

// Len is the number of children, or of inline positions.
func (s Slot) Len() int {
	if s.inline {
		return s.Content().Len()
	}
	return len(s.children)
}

// IndexOf returns where child stands, found by its order key.
func (s Slot) IndexOf(child *Node) (int, bool) {
	i, ok := slices.BinarySearchFunc(s.children, child.ord, func(n *Node, ord string) int {
		return strings.Compare(n.ord, ord)
	})
	if ok && s.children[i].id == child.id {
		return i, true
	}
	for j, n := range s.children {
		if n.id == child.id {
			return j, true
		}
	}
	return 0, false
}

// Node is one node of the tree. It is immutable once published.
type Node struct {
	id    ID
	kind  Kind
	attrs Attrs
	slots []Slot
	// ord is the node's fractional order key among its siblings in a block
	// slot. Inline nodes have none.
	ord string
}

// NewNode builds a node to insert. Its id and order key are given when it
// is inserted.
func NewNode(kind Kind, attrs Attrs, slots ...Slot) *Node {
	return &Node{kind: kind, attrs: attrs, slots: slots}
}

// NewNodeWithID builds a node with a known id, as storage does.
func NewNodeWithID(id ID, kind Kind, attrs Attrs, ord string, slots ...Slot) *Node {
	return &Node{id: id, kind: kind, attrs: attrs, slots: slots, ord: ord}
}

func (n *Node) ID() ID          { return n.id }
func (n *Node) Kind() Kind      { return n.kind }
func (n *Node) Attrs() Attrs    { return n.attrs }
func (n *Node) Ord() string     { return n.ord }
func (n *Node) NumSlots() int   { return len(n.slots) }
func (n *Node) Slot(i int) Slot { return n.slots[i] }

// Slots returns the node's slots. The slice must not change.
func (n *Node) Slots() []Slot { return n.slots }

// clone is a shallow copy whose slots may then be replaced; the children
// slices stay shared until a transaction takes ownership of one.
func (n *Node) clone() *Node {
	c := *n
	c.slots = slices.Clone(n.slots)
	for i := range c.slots {
		c.slots[i].owned = false
	}
	return &c
}

// Text is the plain text of the node's inline slots, in reading order,
// including the slots of nested inline nodes.
func (n *Node) Text() string {
	var b strings.Builder
	n.writeText(&b)
	return b.String()
}

func (n *Node) writeText(b *strings.Builder) {
	for _, s := range n.slots {
		if !s.inline {
			continue
		}
		s.Content().Each(func(_ int, it Item) bool {
			if it.Node != nil {
				it.Node.writeText(b)
			} else {
				b.WriteString(it.Text)
			}
			return true
		})
	}
}

// Walk calls fn with n and every node below it, block and inline, parents
// before children. fn returning false skips the node's descendants.
func (n *Node) Walk(fn func(n *Node) bool) {
	if !fn(n) {
		return
	}
	for _, s := range n.slots {
		if s.inline {
			s.Content().Nodes(func(_ int, child *Node) bool {
				child.Walk(fn)
				return true
			})
			continue
		}
		for _, child := range s.children {
			child.Walk(fn)
		}
	}
}
